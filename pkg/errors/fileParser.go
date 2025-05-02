package errors

import (
	"fmt"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
)

type ErrNoEC2Instances struct {
	Path string
}

func (e ErrNoEC2Instances) Error() string {
	return fmt.Sprintf("no AWS EC2 instances found in state file: %s", filepath.Base(e.Path))
}

func NewNoEC2Instances(path string) error {
	return ErrNoEC2Instances{Path: path}
}

type ErrParse struct {
	Err error
}

func (e ErrParse) Error() string {
	return fmt.Sprintf("parse error: %v", e.Err)
}

func (e ErrParse) Unwrap() error {
	return e.Err
}

// ErrHCLParseFailure wraps an hcl.Diagnostics from parsing HCL.
type ErrHCLParseFailure struct {
	Diagnostics hcl.Diagnostics
}

func (e ErrHCLParseFailure) Error() string {
	return fmt.Sprintf("failed to parse HCL: %s", e.Diagnostics.Error())
}

func (e ErrHCLParseFailure) Unwrap() error {
	return e.Diagnostics.Errs()[0]
}

// ErrHCLDecodeFailure wraps an hcl.Diagnostics from decoding HCL bodies.
type ErrHCLDecodeFailure struct {
	Diagnostics hcl.Diagnostics
}

func (e ErrHCLDecodeFailure) Error() string {
	return fmt.Sprintf("failed to decode HCL: %s", e.Diagnostics.Error())
}

func (e ErrHCLDecodeFailure) Unwrap() error {
	return e.Diagnostics.Errs()[0]
}

// ErrResourceDecode wraps errors encountered decoding an aws_instance block.
type ErrResourceDecode struct {
	ResourceName string
	Diagnostics  hcl.Diagnostics
}

func (e ErrResourceDecode) Error() string {
	return fmt.Sprintf("resource %q: decode failed: %s", e.ResourceName, e.Diagnostics.Error())
}

func (e ErrResourceDecode) Unwrap() error {
	return e.Diagnostics.Errs()[0]
}

// ErrInvalidTagsType occurs when the `tags` attribute is present but not a map.
type ErrInvalidTagsType struct {
	ResourceName string
}

func (e ErrInvalidTagsType) Error() string {
	return fmt.Sprintf("resource %q: tags must be a map[string]string", e.ResourceName)
}
