package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-github/v40/github"
	. "github.com/onsi/gomega"
	"github.com/phoban01/dynamic-watcher/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestRunnerWebhookReconciler_getRunnerDeployment(t *testing.T) {
	tests := []struct {
		name       string
		wantErr    bool
		beforeFunc func(v *unstructured.Unstructured)
	}{
		{
			name:    "when runner deployment exists in namespace",
			wantErr: false,
			beforeFunc: func(u *unstructured.Unstructured) {
				u.SetNamespace("test")
				u.SetName("test")
			},
		},
		{
			name:    "should error when runner deployment does not exist namespace",
			wantErr: true,
			beforeFunc: func(u *unstructured.Unstructured) {
				u.SetNamespace("none")
				u.SetName("test")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			runner := types.GetRunnerDeploymentObject()
			tt.beforeFunc(runner)

			builder := fakeclient.NewClientBuilder().
				WithObjects(runner)

			r := &RunnerWebhookReconciler{
				Client: builder.Build(),
			}

			obj := &types.HorizontalRunnerAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:    "test",
					GenerateName: "runner",
				},
				Spec: types.HRASpec{
					ScaleTargetRef: types.Target{
						Kind: types.RunnerDeploymentKind,
						Name: "test",
					},
					ScaleUpTriggers: []types.Trigger{
						{
							GitHubEvent: "pull_request",
							Amount:      1,
							Duration:    metav1.Duration{Duration: time.Minute},
						},
					},
				},
			}

			_, err := r.getRunnerDeployment(context.TODO(), obj)
			g.Expect(err != nil).To(Equal(tt.wantErr))
		})
	}
}

func TestRunnerWebhookReconciler_reconcileWebhook(t *testing.T) {
	tests := []struct {
		name         string
		owner        string
		repository   string
		webhook      string
		events       []string
		shouldCreate bool
		wantErr      bool
		beforeFunc   func(r *mockRepositoryService)
	}{
		{
			name:         "creates webhook if it does not exist",
			owner:        "test-org",
			repository:   "test-repo",
			shouldCreate: true,
			wantErr:      false,
			webhook:      "http://test.webhook",
			events:       []string{"pull_request"},
		},
		{
			name:         "updates webhook if it does exist but events are different",
			owner:        "test-org",
			repository:   "test-repo",
			shouldCreate: true,
			wantErr:      false,
			webhook:      "http://test.webhook",
			events:       []string{"pull_request", "push"},
			beforeFunc: func(r *mockRepositoryService) {
				hook := &github.Hook{
					Name:   github.String("web"),
					Events: []string{"pull_request"},
					Config: map[string]interface{}{
						"url": "http://test.webhook",
					},
				}
				r.Hooks = append(r.Hooks, hook)
			},
		},
		{
			name:         "does not update webhook if exists with same events",
			owner:        "test-org",
			repository:   "test-repo",
			shouldCreate: false,
			wantErr:      false,
			webhook:      "http://test.webhook",
			events:       []string{"push"},
			beforeFunc: func(r *mockRepositoryService) {
				hook := &github.Hook{
					Name:   github.String("web"),
					Events: []string{"push"},
					Config: map[string]interface{}{
						"url": "http://test.webhook",
					},
				}
				r.Hooks = append(r.Hooks, hook)
			},
		},
		{
			name:         "creates webhook when existing webhooks do not match",
			owner:        "test-org",
			repository:   "test-repo",
			shouldCreate: true,
			wantErr:      false,
			webhook:      "http://test.webhook",
			events:       []string{"push"},
			beforeFunc: func(r *mockRepositoryService) {
				hook := &github.Hook{
					Events: []string{"push"},
					Config: map[string]interface{}{
						"url": "http://existing.test.webhook",
					},
				}
				r.Hooks = append(r.Hooks, hook)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			builder := fakeclient.NewClientBuilder()

			mockRepositoryService := newMockRepositoryService()

			if tt.beforeFunc != nil {
				tt.beforeFunc(mockRepositoryService)
			}

			r := &RunnerWebhookReconciler{
				Client:        builder.Build(),
				Repository:    mockRepositoryService,
				WebhookURL:    tt.webhook,
				WebhookEvents: tt.events,
			}

			err := r.reconcileWebhook(context.TODO(), zap.New(), tt.owner, tt.repository)
			g.Expect(err != nil).To(Equal(tt.wantErr))
			if tt.shouldCreate {
				g.Expect(mockRepositoryService.Hooks).
					Should(ContainElement(&github.Hook{
						Events: tt.events,
						Config: map[string]interface{}{
							"url":          tt.webhook,
							"content_type": "json",
						},
						Active: github.Bool(true),
					}))
			}
		})
	}
}

// mockRepositoryService implements the Repository interface
type mockRepositoryService struct {
	Hooks []*github.Hook
}

// newMockRepositoryService returns a new mockRepositoryService
func newMockRepositoryService(hooks ...*github.Hook) *mockRepositoryService {
	r := &mockRepositoryService{}
	for _, hook := range hooks {
		r.Hooks = append(r.Hooks, hook)
	}
	return r
}

func (r *mockRepositoryService) CreateHook(ctx context.Context, owner, repo string, hook *github.Hook) (*github.Hook, *github.Response, error) {
	var found bool
	for i, h := range r.Hooks {
		if h.Config["url"] == hook.Config["url"] {
			r.Hooks[i] = hook
			found = true
		}
	}
	if !found {
		r.Hooks = append(r.Hooks, hook)
	}
	return nil, nil, nil
}

func (r *mockRepositoryService) ListHooks(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.Hook, *github.Response, error) {
	return r.Hooks, nil, nil
}
