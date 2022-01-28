package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-github/v40/github"
	"github.com/phoban01/dynamic-watcher/pkg/types"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type Repository interface {
	CreateHook(ctx context.Context, owner, repo string, hook *github.Hook) (*github.Hook, *github.Response, error)
	ListHooks(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.Hook, *github.Response, error)
}

// RunnerWebhookReconciler reconciles a BosunHttpHealthCheckWorker object
type RunnerWebhookReconciler struct {
	client.Client
	Repository
	Scheme        *runtime.Scheme
	Events        record.EventRecorder
	WebhookURL    string
	WebhookEvents []string
	syncPeriod    time.Duration
}

// RunnerWebhookReconcilerOptions contains controller configuration
type RunnerWebhookReconcilerOptions struct {
	MaxConcurrentReconciles int
	SyncPeriod              time.Duration
}

// SetupWithManager sets up the controller with the Manager.
func (r *RunnerWebhookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return r.SetupWithManagerAndOptions(mgr, RunnerWebhookReconcilerOptions{})
}

// SetupWithManagerAndOptions sets up the controller with the Manager.
func (r *RunnerWebhookReconciler) SetupWithManagerAndOptions(mgr ctrl.Manager, opts RunnerWebhookReconcilerOptions) error {
	r.syncPeriod = opts.SyncPeriod

	duration := 1 * time.Second
	maxDelay := 120 * time.Second
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(duration, maxDelay)

	return ctrl.NewControllerManagedBy(mgr).
		For(types.GetHorizontalRunnerAutoscalerObject()).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: opts.MaxConcurrentReconciles}).
		WithOptions(controller.Options{RateLimiter: rateLimiter}).
		Complete(r)
}

// Reconcile reconciles GitHub webhooks for HorizontalRunnerAutoscalers
func (r *RunnerWebhookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	logger := logr.FromContext(ctx)

	obj := types.GetHorizontalRunnerAutoscalerObject()

	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		r.Events.Eventf(obj, "Warning", "GetFailed", "Failed to get %s: %v", types.HorizontalRunnerAutoscalerKind, err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log := logger.WithValues(
		"kind", obj.GetKind(),
		"gen", obj.GetGeneration(),
		"namespace", obj.GetNamespace(),
		"name", obj.GetNamespace(),
	)

	log.Info("reconciling")

	data, err := obj.MarshalJSON()
	if err != nil {
		log.Error(err, "Failed to marshal object")
		return ctrl.Result{}, err
	}

	hra := &types.HorizontalRunnerAutoscaler{}
	err = json.Unmarshal(data, hra)
	if err != nil {
		log.Error(err, "Failed to unmarshal object")
		return ctrl.Result{}, err
	}

	if !hra.HasWebhooks() {
		log.Info("No webhook configured")
		return ctrl.Result{}, nil
	}

	runner, err := r.getRunnerDeployment(ctx, hra)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("repository", "repository", runner.GetRepository())

	if err := r.reconcileWebhook(ctx, log, runner.GetOwner(), runner.GetRepository()); err != nil {
		log.Error(err, "Failed to reconcile webhook")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: r.syncPeriod}, nil
}

// getRunnerDeployment returns the runner deployment for the given HorizontalRunnerAutoscaler
func (r *RunnerWebhookReconciler) getRunnerDeployment(ctx context.Context, hra *types.HorizontalRunnerAutoscaler) (*types.RunnerDeployment, error) {
	key := client.ObjectKey{
		Namespace: hra.ObjectMeta.Namespace,
		Name:      hra.Spec.ScaleTargetRef.Name,
	}

	obj := types.GetRunnerDeploymentObject()

	if err := r.Get(ctx, key, obj); err != nil {
		return nil, err
	}

	data, err := obj.MarshalJSON()
	if err != nil {
		return nil, err
	}

	runnerDeployment := &types.RunnerDeployment{}
	err = json.Unmarshal(data, runnerDeployment)
	if err != nil {
		return nil, err
	}

	return runnerDeployment, nil
}

// reconcileWebhook creates or updates the webhook
func (r *RunnerWebhookReconciler) reconcileWebhook(ctx context.Context, log logr.Logger, owner, repository string) error {
	// list repo webhooks
	hooks, _, err := r.ListHooks(ctx, owner, repository, nil)
	if err != nil {
		return fmt.Errorf("error listing webhooks for repository: %w", err)
	}

	// if the hook already exists and the events match, do nothing
	for _, hook := range hooks {
		if hook.Config["url"] == r.WebhookURL && equal(hook.Events, r.WebhookEvents) {
			return nil
		}
	}

	// create the hook if it doesn't exist
	hook, _, err := r.CreateHook(ctx, owner, repository, &github.Hook{
		Active: github.Bool(true),
		Events: r.WebhookEvents,
		Config: map[string]interface{}{
			"url":          r.WebhookURL,
			"content_type": "json",
		},
	})
	if err != nil {
		return fmt.Errorf("error creating webhook: %w", err)
	}

	log.Info("created webhook", "url", hook.GetURL())

	return nil
}

// equal compares slices of strings
func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
