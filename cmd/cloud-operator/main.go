package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrlrt "sigs.k8s.io/controller-runtime"
	ctrlrtcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlrtlog "sigs.k8s.io/controller-runtime/pkg/log"

	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	ctrlnamespace "github.com/temporalio/cloud-operator/controller/namespace"
	"github.com/temporalio/cloud-operator/pkg/version"
	"github.com/temporalio/kube-temporal/pkg/controller/config"
)

const (
	binaryName = "cloud-operator"
)

var (
	flagset  pflag.FlagSet
	scheme   = runtime.NewScheme()
	setupLog = ctrlrt.Log.WithName("setup")
)

func init() {
	_ = tcloudv1alpha1.AddToScheme(scheme)
}

func main() {
	cfg := config.New(config.WithFlags(&flagset))
	pflag.Parse()

	cfg.SetDefaults()
	cfg.SetupLogging()

	setupLog.Info(
		fmt.Sprintf("setting up %s", binaryName),
		"binary_git_version", version.GitVersion,
		"binary_git_commit", version.GitCommit,
		"binary_build_date", version.BuildDate,
	)

	setupLog.Info("initializing controller manager")
	mgr, err := ctrlrt.NewManager(
		ctrlrt.GetConfigOrDie(),
		ctrlrt.Options{
			Scheme: scheme,
			Cache: ctrlrtcache.Options{
				Scheme: scheme,
			},
			HealthProbeBindAddress: cfg.Healthz.BindAddress,
			LeaderElection:         cfg.LeaderElection.Enabled,
			LeaderElectionID:       binaryName,
			Metrics:                cfg.Metrics.ToMetricsServerOptions(),
		},
	)
	if err != nil {
		setupLog.Error(err, "failed creating controller manager")
		os.Exit(1)
	}

	rootLogger := slog.New(logr.ToSlogHandler(ctrlrtlog.Log))

	setupLog.Info("initializing controller")
	c := ctrlnamespace.New(cfg, rootLogger)

	if err = c.BindManager(mgr); err != nil {
		setupLog.Error(
			err,
			"failed binding controller to controller manager",
			"controller", c.Name(),
		)
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "failed setting up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "failed setting up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting controller manager")
	if err := mgr.Start(ctrlrt.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "failed starting controller manager")
		os.Exit(1)
	}
}
