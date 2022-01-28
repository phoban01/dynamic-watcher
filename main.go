package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v40/github"
	"github.com/phoban01/dynamic-watcher/controllers"
	"go.uber.org/zap/zapcore"
	"golang.org/x/oauth2"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	uzap "go.uber.org/zap"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	// Build holds the build sha
	Build string
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}
func main() {
	var (
		metricsAddr             string
		enableLeaderElection    bool
		probeAddr               string
		maxConcurrentReconciles int
		syncPeriod              time.Duration
		webhookURL              string
		webhookEvents           string
	)

	// controller flags
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&maxConcurrentReconciles, "max-concurrent-reconciles", 1, "")
	flag.DurationVar(&syncPeriod, "sync-period", time.Hour*4, "The controller reconcile period.")
	flag.StringVar(&webhookURL, "webhook-url", "https://62bb3ef8c1446b.lhr.domains", "The webhook url to target for this environment")
	flag.StringVar(&webhookEvents, "webhook-events", "workflow_job", "The webhook events to enable")

	// configure logging
	encodeConfig := uzap.NewProductionEncoderConfig()
	encodeConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zap.Encoder(zapcore.NewJSONEncoder(encodeConfig))

	logOpts := zap.Options{
		Development: false,
	}

	logOpts.BindFlags(flag.CommandLine)

	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&logOpts), encoder))

	// check if GITHUB_TOKEN envvar exists
	if os.Getenv("GITHUB_TOKEN") == "" {
		setupLog.Error(fmt.Errorf("GITHUB_TOKEN envvar is not set"), "")
		os.Exit(1)
	}

	// create a new github client using oauth2 authentication
	ctx := ctrl.SetupSignalHandler()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)

	// create oauth2 client
	tc := oauth2.NewClient(ctx, ts)

	// create github client
	ghClient := github.NewClient(tc)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "58a9eb7f.bosun.jspaas.uk",
		Logger:                 ctrl.Log,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	opts := controllers.RunnerWebhookReconcilerOptions{
		MaxConcurrentReconciles: maxConcurrentReconciles,
		SyncPeriod:              syncPeriod,
	}

	if err := (&controllers.RunnerWebhookReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Repository:    ghClient.Repositories,
		WebhookURL:    webhookURL,
		WebhookEvents: strings.Split(webhookEvents, ","),
	}).SetupWithManagerAndOptions(mgr, opts); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RunnerWebhookReconciler")
		os.Exit(1)
	}

	msg := fmt.Sprintf("starting manager @%s", Build)

	setupLog.Info(msg)

	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
