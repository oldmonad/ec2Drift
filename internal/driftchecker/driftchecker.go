package driftchecker

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/oldmonad/ec2Drift/pkg/cloud"
)

// DriftReport contains details about an EC2 instance drift, including
// the instance ID, its name, and a list of drift details that specify
// the attribute that changed and the expected vs actual values.
type DriftReport struct {
	InstanceID string
	Name       string
	Drifts     []DriftDetail
}

// DriftDetail represents an individual change or drift in a specific attribute
// of an EC2 instance, comparing the expected value and the actual value.
type DriftDetail struct {
	Attribute     string
	ExpectedValue interface{}
	ActualValue   interface{}
}

// Detect identifies drifts between two EC2 instance states (old and current).
// It compares the attributes of each instance and returns a list of DriftReports
// for any instance that has changed, including both removed and added instances.
func Detect(
	ctx context.Context,
	oldState []cloud.Instance, // Previous state of the EC2 instances
	currentState []cloud.Instance, // Current state of the EC2 instances
	attributes []string, // List of attributes to check for drift
) []DriftReport {
	// Create maps of EC2 instances by name for fast lookup
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

	// WaitGroup to manage concurrent tasks
	var wg sync.WaitGroup
	// Channel to send drift reports
	reportChan := make(chan DriftReport, len(oldState)+len(currentState))

	// Helper function to send reports to the report channel with context cancellation
	sendReport := func(r DriftReport) {
		select {
		case reportChan <- r:
		case <-ctx.Done():
		}
	}

	// Compare old instances with current ones
	for name, oldInst := range oldMap {
		select {
		case <-ctx.Done():
			break
		default:
		}
		// Check if the current instance exists
		currInst, exists := currMap[name]
		if !exists {
			// If the instance was removed, create a drift report for removal
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

		// If the instance exists, compare the attributes concurrently
		wg.Add(1)
		go func(o, c cloud.Instance, n string) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Initialize an empty list of drift details for each attribute
			drifts := []DriftDetail{}
			for _, attr := range attributes {
				parts := strings.Split(attr, ".")
				switch parts[0] {
				// Check specific attributes for drift
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
					// Compare tags either for specific keys or all keys
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
					// Check root block device attributes (volume size/type)
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
					// Skip unknown attributes
				}
			}

			// If there are any drift details, send a report
			if len(drifts) > 0 {
				sendReport(DriftReport{InstanceID: o.InstanceID, Name: n, Drifts: drifts})
			}
		}(oldInst, currInst, name)
	}

	// Check for instances that exist in the current state but not in the old state (new instances)
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

	// Wait for all goroutines to complete
	wg.Wait()
	// Close the channel after all reports are sent
	close(reportChan)

	// Aggregate results from the report channel into a single list
	driftReports := make([]DriftReport, 0, len(oldState)+len(currentState))
	for rep := range reportChan {
		driftReports = append(driftReports, rep)
	}

	return driftReports
}

// equalStringSlices compares two string slices irrespective of order.
// It sorts and checks if the sorted slices are identical.
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
