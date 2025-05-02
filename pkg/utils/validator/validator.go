package validator

import "github.com/oldmonad/ec2Drift/pkg/parser"

func NewValidator() Validator {
	return &ValidatorOptions{
		validAttributes: map[string]bool{
			"instance_type":                 true,
			"security_groups":               true,
			"ami":                           true,
			"tags":                          true,
			"root_block_device.volume_size": true,
			"root_block_device.volume_type": true,
		},
		supportedFormats: map[string]parser.ParserType{
			"terraform": parser.Terraform,
			"json":      parser.JSON,
		},
	}
}

type ValidatorOptions struct {
	validAttributes  map[string]bool
	supportedFormats map[string]parser.ParserType
}

type Validator interface {
	ValidateAttributes(requested []string) ([]string, error)
	ValidateFormat(format string) (parser.ParserType, error)
}

func NewValidatorOptionsForTesting(validAttrs map[string]bool) *ValidatorOptions {
	return &ValidatorOptions{
		validAttributes:  validAttrs,
		supportedFormats: map[string]parser.ParserType{}, // Default empty map
	}
}
