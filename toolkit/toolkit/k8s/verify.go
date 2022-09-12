package k8s

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/cilium/scaffolding/toolkit/toolkit"
)

// GetNestedSliceStringInterfaceMap returns a slice of string to interface maps within the given unstructured
// at the path described by the given variadic arguments. This is analogous to the suite of "NestedThing"
// functions in apimachinery.
func GetNestedSliceStringInterfaceMap(
	u *unstructured.Unstructured, path ...string,
) ([]map[string]interface{}, bool, error) {
	strPath := strings.Join(path, "/")
	untyped, found, err := unstructured.NestedSlice(u.Object, path...)
	if !found {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, fmt.Errorf("expected slice for %s, got something different: %s", strPath, err)
	}
	typed := []map[string]interface{}{}
	for _, utv := range untyped {
		tv, ok := utv.(map[string]interface{})
		if !ok {
			return nil, true, fmt.Errorf("value in slice %s is not string interface map: %s", strPath, utv)
		}
		typed = append(typed, tv)
	}

	return typed, true, nil
}

// DoesContainerStatusShowCompleted takes the given containerStatus map and returns whether or not the status shows
// the container as having terminated with the reason "Completed".
func DoesContainerStatusShowCompleted(containerStatus map[string]interface{}) (bool, error) {
	reason, reasonFound, err := unstructured.NestedString(
		containerStatus, "state", "terminated", "reason",
	)
	if err != nil {
		return false, err
	}
	if reasonFound && reason == "Completed" {
		return true, nil
	}
	return false, nil
}

// DoesConditionShowReady returns if the given map reflects a condition that is in a ready state.
// For instance, if the condition has "ContainersReady" as true but "PIDPressure" as true as well, the
// condition is showing not ready.
// Essentially just a big switch statement.
func DoesConditionShowReady(condition map[string]interface{}) (bool, error) {
	ct, err := toolkit.PullStrKey("type", condition)
	if err != nil {
		return false, err
	}
	cs, err := toolkit.PullStrKey("status", condition)
	if err != nil {
		return false, err
	}
	switch ct {
	case "Ready", "Initialized", "ContainersReady", "PodScheduled", "Progressing", "Available":
		return cs == string(v1.ConditionTrue), nil
	case "MemoryPressure", "DiskPressure", "PIDPressure", "NetworkUnavailable":
		return cs == string(v1.ConditionFalse), nil
	default:
		return false, fmt.Errorf("unknown condition: %s", ct)
	}
}

// DoesPhaseShowReady is similar to DoesConditionShowReady but works for phases.
// Requires the kind of resource the phase is attached too, as different resource have different phase strings.
// Currently supports Pods, Namespaces and PVCs.
func DoesPhaseShowReady(phase string, kind string) (bool, error) {
	switch kind {
	case "Pod":
		return phase == "Running", nil
	case "Namespace":
		return phase == "Active", nil
	case "PersistentVolumeClaim":
		return phase == "Pending" || phase == "Bound", nil
	}
	return false, fmt.Errorf("kind is either unknown or does not have a phase: %s", kind)
}

// CheckUnstructuredForReadyState takes in an *unstructured.Unstructured and returns a boolean describe if or if not
// the resource should be considered ready, depending on its (if applicable): conditions, phase, containers.
// It also logs the result as a pretty string.
func CheckUnstructuredForReadyState(logger *log.Logger, resource *unstructured.Unstructured) (bool, error) {
	// Gather information
	name := resource.GetName()
	kind := resource.GetKind()
	namespace := resource.GetNamespace()
	phase, phaseFound, err := unstructured.NestedString(resource.Object, "status", "phase")
	if err != nil {
		return false, err
	}
	conditions, conditionsFound, err := GetNestedSliceStringInterfaceMap(resource, "status", "conditions")
	if err != nil {
		return false, err
	}
	containerStatuses, containerStatusesFound, err := GetNestedSliceStringInterfaceMap(
		resource, "status", "containerStatuses",
	)

	// Build info string
	statusStringBuilder := strings.Builder{}
	statusStringBuilder.WriteString(kind + ": " + name + ": ")
	if namespace != "" {
		statusStringBuilder.WriteString("(" + namespace + "): ")
	}

	// Now we do our checks
	resourceIsReady := true
	addComma := true

	if phaseFound && kind != "" {
		addComma = true
		logger.WithFields(log.Fields{
			"phase": phase,
			"kind":  kind,
		}).Debug("evaluating phase")
		statusStringBuilder.WriteString("Phase: " + phase)
		phaseShowsReady, err := DoesPhaseShowReady(phase, kind)
		if err != nil {
			return false, err
		}
		if !phaseShowsReady {
			resourceIsReady = false
		}
	}

	badConditions := []string{}
	if conditionsFound {
		if addComma {
			statusStringBuilder.WriteString(", ")
		}
		addComma = true
		statusStringBuilder.WriteString("Conditions: ")
		logger.WithField("conditions", conditions).Debug("evaluating conditions")
		for i, condition := range conditions {
			if condition["status"].(string) == string(v1.ConditionTrue) {
				statusStringBuilder.WriteString("+")
			} else {
				statusStringBuilder.WriteString("-")
			}
			conditionShowsReady, err := DoesConditionShowReady(condition)
			if err != nil {
				return false, err
			}
			if !conditionShowsReady {
				badConditions = append(badConditions, condition["type"].(string))
				resourceIsReady = false
			}
			statusStringBuilder.WriteString(string(condition["type"].(string)))
			if i < len(conditions)-1 {
				statusStringBuilder.WriteString(" ")
			}
		}
	}

	if containerStatusesFound {
		if addComma {
			statusStringBuilder.WriteString(", ")
		}
		addComma = true
		statusStringBuilder.WriteString("Containers: ")
		for _, containerStatus := range containerStatuses {
			name := containerStatus["name"].(string)
			if containerStatus["ready"].(bool) {
				statusStringBuilder.WriteString("+")
			} else {
				completed, err := DoesContainerStatusShowCompleted(containerStatus)
				if err != nil {
					return false, err
				}
				if completed {
					statusStringBuilder.WriteString("*")
					// mark resource as ready of containers were shown as not ready
					// and no other conditions were impacting pod health
					if len(badConditions) == 2 && toolkit.SliceContains(badConditions, "Ready") {
						resourceIsReady = true
					}
				} else {
					statusStringBuilder.WriteString("-")
					resourceIsReady = false
				}
			}
			statusStringBuilder.WriteString(name + " ")
		}
	}

	statusString := statusStringBuilder.String()
	if resourceIsReady {
		logger.Debug(statusString)
	} else {
		logger.Error(statusString)
	}

	return resourceIsReady, nil
}
