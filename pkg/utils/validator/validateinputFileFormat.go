package validator

import (
	"github.com/oldmonad/ec2Drift/pkg/parser"
)

func (v *ValidatorOptions) ValidateFormat(format string) (parser.ParserType, error) {
	// this is where the file input format would be validated but we
	// would just return the default parser type because there is
	// no support for the alternative, most of the code for
	// extending format type(json) is just for demostration purposes.
	return parser.Terraform, nil
}
