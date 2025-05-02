package validator

import (
	"fmt"
	"sort"

	"github.com/oldmonad/ec2Drift/pkg/errors"
)

// ValidateAttributes checks if all the requested attributes are valid.
// If no attributes are requested, it returns all valid attributes by default.
// If any of the requested attributes are invalid, an error is returned containing
// the list of invalid attributes and the valid attributes.
func (v *ValidatorOptions) ValidateAttributes(requested []string) ([]string, error) {
	// If no attributes are requested, return all valid attributes
	if len(requested) == 0 {
		return v.AllAttributes(), nil
	}

	// Slice to collect any invalid attributes
	var invalidAttrs []string
	for _, a := range requested {
		// Check if the attribute is invalid (not in the valid set)
		if !v.validAttributes[a] {
			invalidAttrs = append(invalidAttrs, a)
		}
	}

	// If there are invalid attributes, return an error containing them
	if len(invalidAttrs) > 0 {
		return nil, &errors.InvalidAttributesError{
			InvalidAttrs: invalidAttrs,
			ValidAttrs:   v.AllAttributes(), // Include all valid attributes for reference
		}
	}

	// Return the requested attributes if all are valid
	return requested, nil
}

// AllAttributes returns a sorted list of all valid attribute names.
func (v *ValidatorOptions) AllAttributes() []string {
	// Create a slice to store all valid attributes
	attributes := make([]string, 0, len(v.validAttributes))

	// Add each valid attribute key to the slice
	for k := range v.validAttributes {
		attributes = append(attributes, k)
	}

	// Sort the attribute names alphabetically
	sort.Strings(attributes)

	// Return the sorted list of valid attributes
	return attributes
}

// FormattedAttributes returns a string representation of all valid attributes,
// formatted with each attribute on a new line, indented with two spaces.
func (v *ValidatorOptions) FormattedAttributes() string {
	var result string
	// Loop through all valid attributes and add each to the result string
	for _, a := range v.AllAttributes() {
		// Format each attribute with a leading bullet point
		result += fmt.Sprintf("  - %s\n", a)
	}
	return result
}
