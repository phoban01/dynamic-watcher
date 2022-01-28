# Runner Webhook Watcher

**Extends functionality provided by https://github.com/actions-runner-controller/actions-runner-controller**

This is a lightweight Kubernetes controller that watches for `HorizontalRunnerAutoscaler`
resources.

When a `HorizontalRunnerAutoscaler` is created with a `scaleTargetRef` the controller
will create or update a webhook on the target GitHub repository.

A Helm chart is included to deploy the controller along with the necessary RBAC.
