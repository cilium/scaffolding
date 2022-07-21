package toolkit

import (
	"context"
	"testing"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDoesConditionShowReadyReturnsCorrectBoolForCondition(t *testing.T) {
	goodConditionTypes := []string{
		"Ready",
		"Initialized",
		"ContainersReady",
		"PodScheduled",
	}
	badConditionTypes := []string{
		"MemoryPressure",
		"DiskPressure",
		"PIDPressure",
	}

	var result bool
	var err error
	condition := GenericCondition{}

	check := func(conditionType string, expectation bool) {
		condition.Type = conditionType
		if expectation {
			condition.Status = string(v1.ConditionTrue)
		} else {
			condition.Status = string(v1.ConditionFalse)
		}
		result, err = DoesConditionShowReady(condition)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		if !result && expectation {
			t.Errorf("expected ready for true condition %s", conditionType)
			t.FailNow()
		}

		if !expectation {
			condition.Status = string(v1.ConditionTrue)
		} else {
			condition.Status = string(v1.ConditionFalse)
		}
		result, err = DoesConditionShowReady(condition)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		if result && !expectation {
			t.Errorf("expected not ready for false condition %s", conditionType)
			t.FailNow()
		}
	}

	for _, goodConditionType := range goodConditionTypes {
		check(goodConditionType, true)
	}
	for _, badConditionType := range badConditionTypes {
		check(badConditionType, false)
	}

}

func TestEnumerateConditionsForReadyStateReturnsCorrectBoolForConditions(t *testing.T) {
	// Assuming TestDoesConditionShowReadyReturnsCorrectBoolForCondition passes,
	// it should not matter what conditions we are using here

	resource := GenericResourceWithConditions{
		Name: "myresource",
		Conditions: []GenericCondition{
			{
				Type:   "DiskPressure",
				Status: string(v1.ConditionFalse),
			},
			{
				Type:   "Ready",
				Status: string(v1.ConditionTrue),
			},
		},
	}

	result, err := EnumerateConditionsForReadyState(
		log.StandardLogger(),
		resource,
		"mycustomresource",
	)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if !result {
		t.Error("expected resource to show ready, yet got not-ready result")
		t.FailNow()
	}

	resource.Conditions[0].Status = string(v1.ConditionTrue)
	result, err = EnumerateConditionsForReadyState(
		log.StandardLogger(),
		resource,
		"mycustomresource",
	)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if result {
		t.Error("expected resource to show not-ready, yet got ready result")
		t.FailNow()
	}

	resource.Conditions[0].Type = "idontexist"
	result, err = EnumerateConditionsForReadyState(
		log.StandardLogger(),
		resource,
		"mycustomresource",
	)
	if err == nil {
		t.Error("expected error for unknown condition type")
	}
	if result {
		t.Error("expected bad resource to show not-ready, yet got ready result")
		t.FailNow()
	}
}

func TestEnumerateResourcesForReadyStateReturnsCorrectBoolForResources(t *testing.T) {
	resources := []GenericResourceWithConditions{
		{
			Name: "gr1",
			Conditions: []GenericCondition{
				{
					Type:   "Ready",
					Status: string(v1.ConditionTrue),
				},
				{
					Type:   "PIDPressure",
					Status: string(v1.ConditionFalse),
				},
			},
		},
		{
			Name: "gr2",
			Conditions: []GenericCondition{
				{
					Type:   "Ready",
					Status: string(v1.ConditionTrue),
				},
				{
					Type:   "PIDPressure",
					Status: string(v1.ConditionFalse),
				},
			},
		},
	}

	result, err := EnumerateResourcesForReadyState(
		log.StandardLogger(),
		resources,
		"mycustomresource",
	)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if !result {
		t.Error("expected resources to show ready, yet got not-ready result")
		t.FailNow()
	}

	resources[0].Conditions[0].Status = string(v1.ConditionFalse)
	result, err = EnumerateResourcesForReadyState(
		log.StandardLogger(),
		resources,
		"mycustomresource",
	)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if result {
		t.Error("expected resources to show not-ready, yet got ready result")
		t.FailNow()
	}

	resources[0].Conditions[0].Type = "idontexist"
	result, err = EnumerateResourcesForReadyState(
		log.StandardLogger(),
		resources,
		"mycustomresource",
	)
	if err == nil {
		t.Error("expected error for unknown condition type")
	}
	if result {
		t.Error("expected bad resources to show not-ready, yet got ready result")
		t.FailNow()
	}
}

func TestVerifyNodesReadyReturnsExpectedAgainstFakeClientset(t *testing.T) {
	ctx := context.TODO()
	logger := log.StandardLogger()
	clientset := fake.NewSimpleClientset()
	nodeApi := clientset.CoreV1().Nodes()

	addNode := func(n *v1.Node) {
		_, err := nodeApi.Create(
			ctx,
			n,
			metav1.CreateOptions{},
		)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
	}

	newNode := func() *v1.Node {
		return &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node" + RandomString(5),
			},
			Status: v1.NodeStatus{
				Conditions: []v1.NodeCondition{
					{
						Type:   "DiskPressure",
						Status: v1.ConditionFalse,
					},
					{
						Type:   "Ready",
						Status: v1.ConditionTrue,
					},
				},
			},
		}
	}
	i := 0
	for i < 2 {
		addNode(newNode())
		i++
	}

	ready, err := VerifyNodesReady(
		ctx, logger, clientset,
	)
	if err != nil {
		t.Error(err)
	}
	if !ready {
		t.Errorf("expected nodes to show ready, instead show not ready")
		t.FailNow()
	}

	badNode := newNode()
	badNode.Status.Conditions[0].Status = v1.ConditionTrue
	addNode(badNode)
	ready, err = VerifyNodesReady(
		ctx, logger, clientset,
	)
	if err != nil {
		t.Error(err)
	}
	if ready {
		t.Errorf("expected nodes to show not ready, instead show ready")
		t.FailNow()
	}

	errorNode := newNode()
	errorNode.Status.Conditions[0].Type = "idontexist"
	addNode(errorNode)
	ready, err = VerifyNodesReady(
		ctx, logger, clientset,
	)
	if err == nil {
		t.Error("expected error to be thrown due to node with bad condition name")
		t.FailNow()
	}
	if ready {
		t.Error("expected ready to be false when error is thrown")
		t.FailNow()
	}
}

func TestPodsReadyReturnsExpectedAgainstFakeClientset(t *testing.T) {
	ctx := context.TODO()
	logger := log.StandardLogger()
	clientset := fake.NewSimpleClientset()
	podApi := clientset.CoreV1().Pods("mynamespace")

	addPod := func(pod *v1.Pod) {
		_, err := podApi.Create(ctx, pod, metav1.CreateOptions{})
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
	}

	newPod := func() *v1.Pod {
		return &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod" + RandomString(5),
			},
			Status: v1.PodStatus{
				Conditions: []v1.PodCondition{
					{
						Type:   "Ready",
						Status: v1.ConditionTrue,
					},
					{
						Type:   "ContainersReady",
						Status: v1.ConditionTrue,
					},
				},
			},
		}
	}

	i := 0
	for i < 2 {
		addPod(newPod())
		i++
	}

	ready, err := VerifyPodsReady(
		ctx, logger, clientset,
	)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if !ready {
		t.Errorf("expected pods to show ready, instead show not ready")
		t.FailNow()
	}

	badPod := newPod()
	badPod.Status.Conditions[0].Status = v1.ConditionFalse
	addPod(badPod)
	ready, err = VerifyPodsReady(
		ctx, logger, clientset,
	)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if ready {
		t.Errorf("expected pods to show not ready, instead show ready")
		t.FailNow()
	}

	errPod := newPod()
	errPod.Status.Conditions[0].Type = "idontexist"
	addPod(errPod)
	ready, err = VerifyPodsReady(
		ctx, logger, clientset,
	)
	if err == nil {
		t.Errorf("expected error to be thrown due to pod with bad condition name")
		t.FailNow()
	}
	if ready {
		t.Errorf("expected ready to be false when error is thrown")

		t.FailNow()
	}
}
