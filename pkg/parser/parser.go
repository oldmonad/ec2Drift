package parser

import (
	"github.com/oldmonad/ec2Drift/pkg/cloud"
)

type Parser interface {
	Parse(content []byte) ([]cloud.Instance, error)
}

type ParserType string

const (
	Terraform ParserType = "terraform"
	JSON      ParserType = "json"
	Unknown   ParserType = "unknown"
)
