package k8s

import (
	"fmt"
	"path/filepath"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	GVRPod = &schema.GroupVersionResource{
		Group:    "",
		Resource: "pods",
		Version:  "v1",
	}
	GVRNode = &schema.GroupVersionResource{
		Group:    "",
		Resource: "nodes",
		Version:  "v1",
	}
	GVRDeployment, _ = schema.ParseResourceArg("deployments.v1.apps")
	GVRNamespace     = &schema.GroupVersionResource{
		Group:    "",
		Resource: "namespaces",
		Version:  "v1",
	}
	GVRPersistentVolumeClaim = &schema.GroupVersionResource{
		Group:    "",
		Resource: "persistentvolumeclaims",
		Version:  "v1",
	}
	GVRConfigMap = &schema.GroupVersionResource{
		Group:    "",
		Resource: "configmaps",
		Version:  "v1",
	}
	GVREvents = &schema.GroupVersionResource{
		Group:    "",
		Resource: "events",
		Version:  "v1",
	}
	GVRCiliumNetworkPolicy = &schema.GroupVersionResource{
		Group:    "cilium.io",
		Resource: "ciliumnetworkpolicies",
		Version:  "v2",
	}
	ResourceToKind = map[string]string{
		GVRPod.Resource:                   "Pod",
		GVRNode.Resource:                  "Node",
		GVRDeployment.Resource:            "Deployment",
		GVRNamespace.Resource:             "Namespace",
		GVRPersistentVolumeClaim.Resource: "PersistentVolumeClaim",
		GVRConfigMap.Resource:             "ConfigMap",
		GVREvents.Resource:                "Event",
		GVRCiliumNetworkPolicy.Resource:   "CiliumNetworkPolicy",
	}
	CatchAllToleration = v1.Toleration{
		Operator: "Exists",
	}
	ScaffoldingLabel         = "cilium.scaffolding"
	ScaffoldingLabelSelector = fmt.Sprintf("app.kubernetes.io=%s", ScaffoldingLabel)
)

// WithScaffoldingLabel adds a scaffolding specific variable to the given unstructured resource.
// This is useful to be able to watch resources created by the scaffolding toolkit.
func WithScaffoldingLabel(res *unstructured.Unstructured) *unstructured.Unstructured {
	res.Object["metadata"].(map[string]interface{})["labels"] = map[string]interface{}{
		"app.kubernetes.io": ScaffoldingLabel,
	}
	return res
}

// NewUnstructuredNS creates a new *unstructured.Unstructured representing a namespace.
// name is the name of the namespace to create.
func NewUnstructuredNS(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": name,
			},
		},
	}
}

// NewUnstructuredPVC creates a new *unstructured.Unstructured representing a PVC.
// It is a simple, classless PVC that depends on defaults setup in the cluster.
func NewUnstructuredPVC(name string, namespace string, accessMode string, size string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "PersistentVolumeClaim",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"accessModes": []string{
					accessMode,
				},
				"resources": map[string]interface{}{
					"requests": map[string]string{
						"storage": size,
					},
				},
			},
		},
	}
}

// UnstructuredPodOpts defines options for NewUnstructuredPod
type UnstructuredPodOpts struct {
	// Pin the pod onto the given node by name
	PinnedNode string
	// Mount the PVC with name "PVCName" into the pod (at /store)
	MountPVC bool
	// Name of PVC to mount into the pod
	PVCName string
	// Mount the config map with the name "ConfigMapName" into the pod (at /configs)
	MountConfigMap bool
	// ConfigMapName is the name of the configmap to mount into the pod
	ConfigMapName string
	// HostNS determines if the pod should enter the node PID and Network NSs and if the pod should be privileged
	HostNS bool
	// WithSleepContainer adds an alpine container to the pod that runs "sleep infinity"
	WithSleepContainer bool
	// HostMounts adds extra host volume mounts into the pod.
	HostMounts []string
	// TolerateAll adds a catch-all toleration to the pod so it is always scheduled.
	TolerateAll bool
}

// NewUnstructuredPod creates a new *unstructured.Unstructured that represents a pod.
func NewUnstructuredPod(
	name string, namespace string, image string, cmd []string, args []string, opts *UnstructuredPodOpts,
) *unstructured.Unstructured {
	volumeMounts := []map[string]interface{}{}
	podContainer := map[string]interface{}{
		"name":  "main",
		"image": image,
		"args":  args,
	}
	sleepContainer := map[string]interface{}{
		"name":  "sleep",
		"image": "alpine",
		"args":  []string{"sleep", "infinity"},
	}
	volumes := []map[string]interface{}{}
	podSpec := map[string]interface{}{
		"terminationGracePeriodSeconds": 1,
		"restartPolicy":                 "Never",
	}
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
		},
	}

	if len(cmd) > 0 {
		podContainer["command"] = cmd
	}
	if opts.PinnedNode != "" {
		podSpec["nodeSelector"] = map[string]interface{}{
			"kubernetes.io/hostname": opts.PinnedNode,
		}
	}
	if opts.HostNS {
		podContainer["securityContext"] = map[string]interface{}{
			"privileged": true,
		}
		podSpec["hostPID"] = true
		podSpec["hostNetwork"] = true
	}
	if opts.MountPVC {
		volumes = append(volumes, map[string]interface{}{
			"name": "store",
			"persistentVolumeClaim": map[string]interface{}{
				"claimName": opts.PVCName,
			},
		})
		volumeMounts = append(volumeMounts, map[string]interface{}{
			"mountPath": "/store",
			"name":      "store",
		})
	}
	if opts.MountConfigMap {
		volumes = append(volumes, map[string]interface{}{
			"name": opts.ConfigMapName,
			"configMap": map[string]interface{}{
				"name":        opts.ConfigMapName,
				"defaultMode": 0700,
			},
		})
		volumeMounts = append(volumeMounts, map[string]interface{}{
			"name":      opts.ConfigMapName,
			"mountPath": "/configs",
		})
	}
	for i, hostPath := range opts.HostMounts {
		n := fmt.Sprintf("host-path-%d", i)
		volumes = append(volumes, map[string]interface{}{
			"name": n,
			"hostPath": map[string]interface{}{
				"path": hostPath,
			},
		})
		volumeMounts = append(volumeMounts, map[string]interface{}{
			"name": n,
			"mountPath": filepath.Join("/host", hostPath),
		})
	}
	if opts.TolerateAll {
		podSpec["tolerations"] = []v1.Toleration{CatchAllToleration}
	}

	podContainer["volumeMounts"] = volumeMounts
	sleepContainer["volumeMounts"] = volumeMounts
	podSpec["containers"] = []map[string]interface{}{podContainer}
	if opts.WithSleepContainer {
		podSpec["containers"] = append(podSpec["containers"].([]map[string]interface{}), sleepContainer)
	}
	podSpec["volumes"] = volumes
	pod.Object["spec"] = podSpec

	return pod
}

// NewUnstructuredConfigMap creates a new *unstructured.Unstructured that represents a configmap
// data is a map that will be added under the configmaps "data" field.
func NewUnstructuredConfigMap(name string, namespace string, data map[string]string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"data": data,
		},
	}
}
