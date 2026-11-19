package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrlrt "sigs.k8s.io/controller-runtime"
	ctrlrtcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlrtlog "sigs.k8s.io/controller-runtime/pkg/log"

	tcloudv1alpha1 "github.com/temporalio/cloud-operator/api/cloud/v1alpha1"
	ctrlapikey "github.com/temporalio/cloud-operator/controller/apikey"
	ctrlnamespace "github.com/temporalio/cloud-operator/controller/namespace"
	ctrlproject "github.com/temporalio/cloud-operator/controller/project"
	ctrlserviceaccount "github.com/temporalio/cloud-operator/controller/serviceaccount"
	ctrluser "github.com/temporalio/cloud-operator/controller/user"
	"github.com/temporalio/cloud-operator/pkg/reconciler/base"
	"github.com/temporalio/cloud-operator/pkg/version"
	ktcontroller "github.com/temporalio/kube-temporal/pkg/controller"
	"github.com/temporalio/kube-temporal/pkg/controller/config"
	tcclient "go.temporal.io/cloud-sdk/cloudclient"
)

const (
	binaryName        = "cloud-operator"
	defaultApiKeyPath = "/etc/temporal-cloud/apikey"
)

var (
	flagset  pflag.FlagSet
	scheme   = runtime.NewScheme()
	setupLog = ctrlrt.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = tcloudv1alpha1.AddToScheme(scheme)
}

func main() {
	var cfg config.Config
	cfg.BindFlags(&flagset)
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

	setupLog.Info("initializing Temporal Cloud client")
	apiKey, err := os.ReadFile(defaultApiKeyPath)
	if err != nil {
		setupLog.Error(err, "failed reading Temporal Cloud API key")
		os.Exit(1)
	}
	tcc, err := tcclient.New(tcclient.Options{
		APIKey: string(apiKey), UserAgent: version.UserAgent(),
	})
	if err != nil {
		setupLog.Error(err, "failed creating Temporal Cloud client")
		os.Exit(1)
	}

	setupLog.Info("initializing controllers")
	controllers := []*ktcontroller.Controller{
		ctrlnamespace.New(
			cfg,
			rootLogger,
			base.WithDefaultCloudClient(*tcc),
		),
		ctrlserviceaccount.New(
			cfg,
			rootLogger,
			base.WithDefaultCloudClient(*tcc),
		),
		ctrlproject.New(
			cfg,
			rootLogger,
			base.WithDefaultCloudClient(*tcc),
		),
		ctrlapikey.New(
			cfg,
			rootLogger,
			base.WithDefaultCloudClient(*tcc),
		),
		ctrluser.New(
			cfg,
			rootLogger,
			base.WithDefaultCloudClient(*tcc),
		),
	}

	for _, c := range controllers {
		if err = c.BindManager(mgr); err != nil {
			setupLog.Error(
				err,
				"failed binding controller to controller manager",
				"controller", c.Name(),
			)
			os.Exit(1)
		}
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
