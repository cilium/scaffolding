package toolkit

import (
	"context"
	"fmt"

	"strings"

	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
)

type GenericCondition struct {
	Type   string
	Status string
}

func DoesConditionShowReady(condition GenericCondition) (bool, error) {
	switch condition.Type {
	case "Ready", "Initialized", "ContainersReady", "PodScheduled", "Progressing", "Available":
		return condition.Status == string(v1.ConditionTrue), nil
	case "MemoryPressure", "DiskPressure", "PIDPressure", "NetworkUnavailable":
		return condition.Status == string(v1.ConditionFalse), nil
	}
	return false, fmt.Errorf("unknown condition: %s", condition.Type)
}

func NewGenericConditionFromNodeCondition(nodeCondition v1.NodeCondition) GenericCondition {
	return GenericCondition{
		Type:   string(nodeCondition.Type),
		Status: string(nodeCondition.Status),
	}
}

func NewGenericConditionFromPodCondition(podCondition v1.PodCondition) GenericCondition {
	return GenericCondition{
		Type:   string(podCondition.Type),
		Status: string(podCondition.Status),
	}
}

func NewGenericConditionFromDeploymentCondition(deploymentCondition appsv1.DeploymentCondition) GenericCondition {
	return GenericCondition{
		Type:   string(deploymentCondition.Type),
		Status: string(deploymentCondition.Status),
	}
}

type GenericResourceWithConditions struct {
	Name       string
	Namespace  string
	Conditions []GenericCondition
}

func NewGenericResourceWithConditionsFromNode(node v1.Node) GenericResourceWithConditions {
	conditions := []GenericCondition{}

	for _, condition := range node.Status.Conditions {
		conditions = append(
			conditions,
			NewGenericConditionFromNodeCondition(condition),
		)
	}

	return GenericResourceWithConditions{
		Name:       node.Name,
		Namespace:  "",
		Conditions: conditions,
	}
}

func NewGenericResourceWithConditionsFromPod(pod v1.Pod) GenericResourceWithConditions {
	conditions := []GenericCondition{}

	for _, condition := range pod.Status.Conditions {
		conditions = append(
			conditions,
			NewGenericConditionFromPodCondition(condition),
		)
	}

	return GenericResourceWithConditions{
		Name:       pod.Name,
		Namespace:  pod.Namespace,
		Conditions: conditions,
	}
}

func NewGenericResourceWithConditionsFromDeployment(deployment appsv1.Deployment) GenericResourceWithConditions {
	conditions := []GenericCondition{}

	for _, condition := range deployment.Status.Conditions {
		conditions = append(
			conditions,
			NewGenericConditionFromDeploymentCondition(condition),
		)
	}

	return GenericResourceWithConditions{
		Name:       deployment.Name,
		Namespace:  deployment.Namespace,
		Conditions: conditions,
	}
}

func EnumerateConditionsForReadyState(
	logger *log.Logger,
	resource GenericResourceWithConditions,
	resourceTypeName string,
) (bool, error) {
	statusStringBuilder := strings.Builder{}
	statusStringBuilder.WriteString(
		resourceTypeName + ": " + resource.Name + ": ",
	)
	if resource.Namespace != "" {
		statusStringBuilder.WriteString(
			"(" + resource.Namespace + "): ",
		)
	}

	resourceIsReady := true
	for _, condition := range resource.Conditions {
		if condition.Status == string(v1.ConditionTrue) {
			statusStringBuilder.WriteString("+")
		} else {
			statusStringBuilder.WriteString("-")
		}
		conditionShowsReady, err := DoesConditionShowReady(condition)
		if err != nil {
			return false, err
		}
		if !conditionShowsReady {
			resourceIsReady = false
		}
		statusStringBuilder.WriteString(string(condition.Type) + " ")
	}

	statusString := statusStringBuilder.String()
	if resourceIsReady {
		logger.Info(statusString)
	} else {
		logger.Error(statusString)
	}

	return resourceIsReady, nil
}

func EnumerateResourcesForReadyState(
	logger *log.Logger,
	resources []GenericResourceWithConditions,
	resourceTypeName string,
) (bool, error) {
	resourcesAreReady := true

	for _, resource := range resources {
		ready, err := EnumerateConditionsForReadyState(logger, resource, resourceTypeName)
		if err != nil {
			return false, err
		}
		if !ready {
			resourcesAreReady = false
		}
	}

	if resourcesAreReady {
		formatStr := "the cluster's %s is ready!"
		if len(resources) > 1 {
			formatStr = "all %ss are ready!"
		}
		logger.Info(
			fmt.Sprintf(
				"💚 "+formatStr,
				resourceTypeName,
			),
		)
	} else {
		formatStr := "the cluster's %s don't look right"
		if len(resources) > 1 {
			formatStr = "looks like not all %ss are ready"
		}
		logger.Error(
			fmt.Sprintf(
				"🤔 "+formatStr,
				resourceTypeName,
			),
		)
	}

	return resourcesAreReady, nil
}

func VerifyNodesReady(ctx context.Context, logger *log.Logger, clientset k8s.Interface) (bool, error) {

	nodes, err := clientset.CoreV1().Nodes().List(
		ctx,
		metav1.ListOptions{},
	)
	if err != nil {
		return false, err
	}

	logger.Info("verifying nodes in ready state")
	if len(nodes.Items) == 0 {
		logger.Warn("it seems the cluster has zero nodes, so, they are all ready?")
		return true, nil
	}

	nodesAsResources := []GenericResourceWithConditions{}
	for _, node := range nodes.Items {
		nodesAsResources = append(
			nodesAsResources,
			NewGenericResourceWithConditionsFromNode(node),
		)
	}
	return EnumerateResourcesForReadyState(
		logger,
		nodesAsResources,
		"node",
	)
}

func VerifyPodsReady(ctx context.Context, logger *log.Logger, clientset k8s.Interface) (bool, error) {

	pods, err := clientset.CoreV1().Pods("").List(
		ctx,
		metav1.ListOptions{},
	)
	if err != nil {
		return false, err
	}

	logger.Info("verifying pods in ready state")
	if len(pods.Items) == 0 {
		logger.Warn("it seems the cluster has zero pods, so, they are all ready?")
	}

	podsAsResources := []GenericResourceWithConditions{}
	for _, pod := range pods.Items {
		podsAsResources = append(
			podsAsResources,
			NewGenericResourceWithConditionsFromPod(pod),
		)
	}
	return EnumerateResourcesForReadyState(
		logger,
		podsAsResources,
		"pod",
	)
}

func VerifyDeploymentsReady(ctx context.Context, logger *log.Logger, clientset k8s.Interface) (bool, error) {
	deployments, err := clientset.AppsV1().Deployments("").List(
		ctx,
		metav1.ListOptions{},
	)
	if err != nil {
		return false, err
	}

	logger.Info("verifying deployments in ready state")
	if len(deployments.Items) == 0 {
		logger.Warn("it seems the cluster has zero deployments, so, they are all ready?")
	}

	deploymentsAsResources := []GenericResourceWithConditions{}
	for _, deployment := range deployments.Items {
		deploymentsAsResources = append(
			deploymentsAsResources,
			NewGenericResourceWithConditionsFromDeployment(deployment),
		)
	}
	return EnumerateResourcesForReadyState(
		logger,
		deploymentsAsResources,
		"deployment",
	)

}
