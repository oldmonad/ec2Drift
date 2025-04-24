package comparator

import (
	"log"
	"strings"
	"sync"

	"github.com/oldmonad/ec2Drift.git/internal/parser"
)

type DriftReport struct {
	InstanceID      string
	ApplicationName string
	Drifts          []DriftDetail
}

type DriftDetail struct {
	Attribute     string
	ExpectedValue interface{}
	ActualValue   interface{}
}

func DetectDrift(
	stateInstances []parser.Instance,
	configs []*parser.InstanceConfig,
	attributes []string,
) []DriftReport {
	var reports []DriftReport
	var wg sync.WaitGroup
	reportChan := make(chan DriftReport)

	for _, instance := range stateInstances {
		wg.Add(1)
		go func(inst parser.Instance) {
			defer wg.Done()
			baseName := extractBaseName(inst.IndexKey)

			var config *parser.InstanceConfig
			for _, cfg := range configs {
				if cfg.ApplicationName == baseName {
					config = cfg
					break
				}
			}

			if config == nil {
				log.Printf("No matching config found for instance: %s", inst.IndexKey)
				return
			}

			drifts := compareAttributes(inst.Attributes, config, attributes)
			if len(drifts) > 0 {
				reportChan <- DriftReport{
					InstanceID:      inst.Attributes.ID,
					ApplicationName: baseName,
					Drifts:          drifts,
				}
			}
		}(instance)
	}

	go func() {
		wg.Wait()
		close(reportChan)
	}()

	for report := range reportChan {
		reports = append(reports, report)
	}

	return reports
}

func extractBaseName(indexKey string) string {
	lastHyphen := strings.LastIndex(indexKey, "-")
	if lastHyphen == -1 {
		return indexKey
	}
	return indexKey[:lastHyphen]
}

func compareAttributes(
	stateAttrs parser.InstanceAttributes,
	config *parser.InstanceConfig,
	attributes []string,
) []DriftDetail {
	var drifts []DriftDetail

	for _, attr := range attributes {
		switch attr {
		case "application_name", "no_of_instances":
			continue

		case "instance_type":
			if config.InstanceType != stateAttrs.InstanceType {
				drifts = append(drifts, createDrift(attr, config.InstanceType, stateAttrs.InstanceType))
			}

		case "source_dest_check":
			if config.SourceDestCheck == nil {
				continue
			}

			if *config.SourceDestCheck != stateAttrs.SourceDestCheck {
				drifts = append(drifts, createDrift(attr, *config.SourceDestCheck, stateAttrs.SourceDestCheck))
			}

		case "subnet_id":
			if config.SubnetID == nil {
				continue
			}

			if *config.SubnetID != stateAttrs.SubnetID {
				drifts = append(drifts, createDrift(attr, *config.SubnetID, stateAttrs.SubnetID))
			}

		case "security_groups":
			if config.SecurityGroups == nil {
				continue
			}

			if len(config.SecurityGroups) != len(stateAttrs.SecurityGroups) {
				drifts = append(drifts, createDrift(attr, config.SecurityGroups, stateAttrs.SecurityGroups))
			} else {
				for i, group := range config.SecurityGroups {
					if group != stateAttrs.SecurityGroups[i] {
						drifts = append(drifts, createDrift(attr, group, stateAttrs.SecurityGroups[i]))
					}
				}
			}

		case "vpc_security_group_ids":
			if config.VPCSecurityGroupIDs == nil {
				continue
			}

			if len(config.VPCSecurityGroupIDs) != len(stateAttrs.VPCSecurityGroupIDs) {
				drifts = append(drifts, createDrift(attr, config.VPCSecurityGroupIDs, stateAttrs.VPCSecurityGroupIDs))
			} else {
				for i, group := range config.VPCSecurityGroupIDs {
					if group != stateAttrs.VPCSecurityGroupIDs[i] {
						drifts = append(drifts, createDrift(attr, group, stateAttrs.VPCSecurityGroupIDs[i]))
					}
				}
			}

		case "ami":
			if config.AMI != stateAttrs.AMI {
				drifts = append(drifts, createDrift(attr, config.AMI, stateAttrs.AMI))
			}

		default:
			continue
		}
	}

	return drifts
}

func createDrift(attr string, expected, actual interface{}) DriftDetail {
	return DriftDetail{
		Attribute:     attr,
		ExpectedValue: expected,
		ActualValue:   actual,
	}
}
