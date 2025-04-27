package driftchecker

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/oldmonad/ec2Drift/pkg/cloud"
)

type DriftReport struct {
	InstanceID string
	Name       string
	Drifts     []DriftDetail
}

type DriftDetail struct {
	Attribute     string
	ExpectedValue interface{}
	ActualValue   interface{}
}

func Detect(
	ctx context.Context,
	oldState []cloud.Instance,
	currentState []cloud.Instance,
	attributes []string,
) []DriftReport {
	oldMap := make(map[string]cloud.Instance, len(oldState))
	for _, inst := range oldState {
		if name, ok := inst.Tags["Name"]; ok {
			oldMap[name] = inst
		}
	}
	currMap := make(map[string]cloud.Instance, len(currentState))
	for _, inst := range currentState {
		if name, ok := inst.Tags["Name"]; ok {
			currMap[name] = inst
		}
	}

	var wg sync.WaitGroup
	reportChan := make(chan DriftReport, len(oldState)+len(currentState))

	sendReport := func(r DriftReport) {
		select {
		case reportChan <- r:
		case <-ctx.Done():
		}
	}

	for name, oldInst := range oldMap {
		select {
		case <-ctx.Done():
			break
		default:
		}
		currInst, exists := currMap[name]
		if !exists {
			wg.Add(1)
			go func(o cloud.Instance, n string) {
				defer wg.Done()
				select {
				case <-ctx.Done():
					return
				default:
				}

				sendReport(DriftReport{
					InstanceID: o.InstanceID,
					Name:       n,
					Drifts: []DriftDetail{{
						Attribute:     "instance_removed",
						ExpectedValue: o,
						ActualValue:   nil,
					}},
				})
			}(oldInst, name)
			continue
		}

		wg.Add(1)
		go func(o, c cloud.Instance, n string) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			default:
			}

			drifts := []DriftDetail{}
			for _, attr := range attributes {
				parts := strings.Split(attr, ".")
				switch parts[0] {
				case "ami":
					if o.AMI != c.AMI {
						drifts = append(drifts, DriftDetail{attr, o.AMI, c.AMI})
					}
				case "instance_type":
					if o.InstanceType != c.InstanceType {
						drifts = append(drifts, DriftDetail{attr, o.InstanceType, c.InstanceType})
					}
				case "security_groups":
					if !equalStringSlices(o.SecurityGroups, c.SecurityGroups) {
						drifts = append(drifts, DriftDetail{attr, o.SecurityGroups, c.SecurityGroups})
					}
				case "tags":
					if len(parts) > 1 {
						key := parts[1]
						if key == "Name" {
							continue
						}
						oVal, oOk := o.Tags[key]
						cVal, cOk := c.Tags[key]
						if !oOk || !cOk || oVal != cVal {
							drifts = append(drifts, DriftDetail{attr, oVal, cVal})
						}
					} else {
						for k, ov := range o.Tags {
							if k == "Name" {
								continue
							}
							cv, ok := c.Tags[k]
							if !ok || ov != cv {
								drifts = append(drifts, DriftDetail{"tags." + k, ov, cv})
							}
						}
					}
				case "root_block_device":
					if len(parts) > 1 {
						sub := parts[1]
						switch sub {
						case "volume_size":
							if o.RootBlockDevice.VolumeSize != c.RootBlockDevice.VolumeSize {
								drifts = append(drifts, DriftDetail{attr, o.RootBlockDevice.VolumeSize, c.RootBlockDevice.VolumeSize})
							}
						case "volume_type":
							if o.RootBlockDevice.VolumeType != c.RootBlockDevice.VolumeType {
								drifts = append(drifts, DriftDetail{attr, o.RootBlockDevice.VolumeType, c.RootBlockDevice.VolumeType})
							}
						}
					} else {
						if o.RootBlockDevice.VolumeSize != c.RootBlockDevice.VolumeSize {
							drifts = append(drifts, DriftDetail{"root_block_device.volume_size", o.RootBlockDevice.VolumeSize, c.RootBlockDevice.VolumeSize})
						}
						if o.RootBlockDevice.VolumeType != c.RootBlockDevice.VolumeType {
							drifts = append(drifts, DriftDetail{"root_block_device.volume_type", o.RootBlockDevice.VolumeType, c.RootBlockDevice.VolumeType})
						}
					}
				default:
					// unknown attribute: skip
				}
			}

			if len(drifts) > 0 {
				sendReport(DriftReport{InstanceID: o.InstanceID, Name: n, Drifts: drifts})
			}
		}(oldInst, currInst, name)
	}

	for name, currInst := range currMap {
		if _, exists := oldMap[name]; !exists {
			wg.Add(1)
			go func(c cloud.Instance, n string) {
				defer wg.Done()
				select {
				case <-ctx.Done():
					return
				default:
				}

				sendReport(DriftReport{InstanceID: c.InstanceID, Name: n, Drifts: []DriftDetail{{
					Attribute:     "instance_added",
					ExpectedValue: nil,
					ActualValue:   c,
				}}})
			}(currInst, name)
		}
	}

	wg.Wait()
	close(reportChan)

	// Aggregate results
	driftReports := make([]DriftReport, 0, len(oldState)+len(currentState))
	for rep := range reportChan {
		driftReports = append(driftReports, rep)
	}

	return driftReports
}

// equalStringSlices compares two string slices irrespective of order.
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aCopy := append([]string(nil), a...)
	bCopy := append([]string(nil), b...)
	sort.Strings(aCopy)
	sort.Strings(bCopy)
	return reflect.DeepEqual(aCopy, bCopy)
}
