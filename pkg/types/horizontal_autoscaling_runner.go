package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	HorizontalRunnerAutoscalerGroup   = "actions.summerwind.dev"
	HorizontalRunnerAutoscalerVersion = "v1alpha1"
	HorizontalRunnerAutoscalerKind    = "HorizontalRunnerAutoscaler"
)

// GetHorizontalRunnerAutoscalerObject returns an unstructured HorizontalRunnerAutoscaler object
func GetHorizontalRunnerAutoscalerObject() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   HorizontalRunnerAutoscalerGroup,
		Version: HorizontalRunnerAutoscalerVersion,
		Kind:    HorizontalRunnerAutoscalerKind,
	})
	return obj
}

// HorizontalRunnerAutoscaler is the Schema for the horizontal runner autoscalers
type HorizontalRunnerAutoscaler struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              HRASpec `json:"spec"`
}

// HRASpec is the spec for the horizontal runner autoscaler
type HRASpec struct {
	ScaleTargetRef  Target    `json:"scaleTargetRef"`
	ScaleUpTriggers []Trigger `json:"scaleUpTriggers"`
}

// Target is the target for the horizontal runner autoscaler
type Target struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// Trigger is the trigger for the horizontal runner autoscaler
type Trigger struct {
	GitHubEvent interface{}     `json:"githubEvent,omitempty"`
	Amount      int             `json:"amount,omitempty"`
	Duration    metav1.Duration `json:"duration,omitempty"`
}

// HasWebhooks returns true if the horizontal runner autoscaler has scaleUpTriggers
func (h *HorizontalRunnerAutoscaler) HasWebhooks() bool {
	return len(h.Spec.ScaleUpTriggers) > 0
}
