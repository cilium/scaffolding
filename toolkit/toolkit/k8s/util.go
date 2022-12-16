package k8s

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewListOptionsFromName constructs a new ListOptions that uses a FieldSelector
// to target resources whose metadata.name attribute matches the given name.
func NewListOptionsFromName(name string) metav1.ListOptions {
	return metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", name),
	}
}
