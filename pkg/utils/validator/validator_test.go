package validator_test

import (
	"testing"

	"github.com/oldmonad/ec2Drift/pkg/errors"
	"github.com/oldmonad/ec2Drift/pkg/parser"
	"github.com/oldmonad/ec2Drift/pkg/utils/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAttributes(t *testing.T) {
	v := validator.NewValidator()

	t.Run("empty requested attributes returns all valid attributes sorted", func(t *testing.T) {
		expected := []string{
			"ami",
			"instance_type",
			"root_block_device.volume_size",
			"root_block_device.volume_type",
			"security_groups",
			"tags",
		}

		attrs, err := v.ValidateAttributes([]string{})
		require.NoError(t, err)
		assert.Equal(t, expected, attrs)
	})

	t.Run("valid requested attributes returns the same attributes", func(t *testing.T) {
		requested := []string{"ami", "security_groups", "tags"}

		attrs, err := v.ValidateAttributes(requested)
		require.NoError(t, err)
		assert.Equal(t, requested, attrs)
	})

	t.Run("some invalid attributes returns error with invalid list", func(t *testing.T) {
		requested := []string{"ami", "invalid_attr", "another_invalid"}

		attrs, err := v.ValidateAttributes(requested)
		require.Error(t, err)
		assert.Nil(t, attrs)

		invalidErr, ok := err.(*errors.InvalidAttributesError)
		require.True(t, ok, "error should be of type InvalidAttributesError")

		expectedInvalid := []string{"invalid_attr", "another_invalid"}
		assert.Equal(t, expectedInvalid, invalidErr.InvalidAttrs)

		expectedValid := []string{
			"ami",
			"instance_type",
			"root_block_device.volume_size",
			"root_block_device.volume_type",
			"security_groups",
			"tags",
		}
		assert.Equal(t, expectedValid, invalidErr.ValidAttrs)
	})

	t.Run("all invalid attributes returns error with all invalid listed", func(t *testing.T) {
		requested := []string{"invalid1", "invalid2"}

		attrs, err := v.ValidateAttributes(requested)
		require.Error(t, err)
		assert.Nil(t, attrs)

		invalidErr := err.(*errors.InvalidAttributesError)
		assert.Equal(t, requested, invalidErr.InvalidAttrs)

		// Access via concrete type to get valid attributes
		vo := v.(*validator.ValidatorOptions) // Type assertion
		assert.Equal(t, vo.AllAttributes(), invalidErr.ValidAttrs)
	})
}

func TestValidateFormat(t *testing.T) {
	v := validator.NewValidator()

	tests := []struct {
		name         string
		inputFormat  string
		expectedType parser.ParserType
	}{
		{
			name:         "terraform format returns Terraform parser",
			inputFormat:  "terraform",
			expectedType: parser.Terraform,
		},
		{
			name:         "json format returns Terraform parser",
			inputFormat:  "json",
			expectedType: parser.Terraform,
		},
		{
			name:         "empty format returns Terraform parser",
			inputFormat:  "",
			expectedType: parser.Terraform,
		},
		{
			name:         "unsupported format returns Terraform parser",
			inputFormat:  "yaml",
			expectedType: parser.Terraform,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parserType, err := v.ValidateFormat(tt.inputFormat)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedType, parserType)
		})
	}
}

func TestFormattedAttributes(t *testing.T) {
	t.Run("formats valid attributes with hyphens and newlines", func(t *testing.T) {
		vo := validator.NewValidator().(*validator.ValidatorOptions) // Type assertion to access unexported method

		// Expected output matches the sorted attributes with formatting
		expected := `  - ami
  - instance_type
  - root_block_device.volume_size
  - root_block_device.volume_type
  - security_groups
  - tags
`
		assert.Equal(t, expected, vo.FormattedAttributes())
	})

	t.Run("returns empty string when there are no valid attributes", func(t *testing.T) {
		vo := validator.NewValidatorOptionsForTesting(map[string]bool{})
		assert.Empty(t, vo.FormattedAttributes())
	})
}
