package parser

import (
	"encoding/json"

	"github.com/oldmonad/ec2Drift/pkg/cloud"
)

type JSONParser struct{}

func (p *JSONParser) Parse(content []byte) ([]cloud.Instance, error) {
	var instances []cloud.Instance
	if err := json.Unmarshal(content, &instances); err != nil {
		return nil, err
	}
	return instances, nil
}
