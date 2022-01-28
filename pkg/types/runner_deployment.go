package types

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	RunnerDeploymentGroup   = "actions.summerwind.dev"
	RunnerDeploymentVersion = "v1alpha1"
	RunnerDeploymentKind    = "RunnerDeployment"
)

func GetRunnerDeploymentObject() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   RunnerDeploymentGroup,
		Version: RunnerDeploymentVersion,
		Kind:    RunnerDeploymentKind,
	})
	return obj
}

type RunnerDeployment struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DeploymentSpec `json:"spec"`
}

type DeploymentSpec struct {
	Template RunnerTemplate `json:"template"`
}

type RunnerTemplate struct {
	Spec RunnerSpec `json:"spec"`
}

type RunnerSpec struct {
	Repository string `json:"repository"`
}

// GetRepository returns the repository of the runner deployment
func (r *RunnerDeployment) GetRepository() string {
	parts := strings.Split(r.Spec.Template.Spec.Repository, "/")
	if len(parts) == 1 {
		return r.Spec.Template.Spec.Repository
	}

	return parts[len(parts)-1]
}

// GetOwner returns the owner of the runner deployment
func (r *RunnerDeployment) GetOwner() string {
	parts := strings.Split(r.Spec.Template.Spec.Repository, "/")
	if len(parts) == 1 {
		return ""
	}

	return parts[0]
}
