package k8s

import (
	"fmt"
)

// NewFieldSelectorFromName constructs a new field selector
// to target resources whose metadata.name attribute matches the given name.
func NewFieldSelectorFromName(name string) string {
	return fmt.Sprintf("metadata.name=%s", name)
}
