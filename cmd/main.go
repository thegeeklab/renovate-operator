package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/controller/discovery"
	"github.com/thegeeklab/renovate-operator/internal/controller/gitrepo"
	"github.com/thegeeklab/renovate-operator/internal/controller/renovator"
	runner "github.com/thegeeklab/renovate-operator/internal/controller/runner"
	"github.com/thegeeklab/renovate-operator/internal/frontend"
	"github.com/thegeeklab/renovate-operator/internal/receiver"
	webhookrenovatev1beta1 "github.com/thegeeklab/renovate-operator/internal/webhook/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	errWebhookTimeout = errors.New("timeout waiting for webhook")
	errFlagRequired   = errors.New("missing required flag")
)

const (
	webhookCAName         = "renovate-operator-ca"
	webhookCAOrganization = "renovate-operator"
	webhookName           = "renovate-operator-webhook-configuration"
	webhookSecretName     = "renovate-operator-webhook-server-cert"
	webhookCertService    = "renovate-operator-webhook-service"
)

// Namespace Scoped
//nolint:lll
// +kubebuilder:rbac:groups=coordination.k8s.io,namespace=system,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,namespace=system,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,namespace=system,resources=secrets,verbs=create;delete;get;update;patch;list;watch

// Cluster Scoped
//nolint:lll
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations,verbs=create;delete;get;update;patch;list;watch
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=create;delete;get;update;patch;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

//nolint:wsl
func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(renovatev1beta1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// Config holds all command-line configuration for the operator.
type Config struct {
	MetricsAddr          string
	EnableLeaderElection bool
	ProbeAddr            string
	SecureMetrics        bool
	WebhookCertRotation  bool
	WebhookCertPath      string
	EnableHTTP2          bool
	WatchNamespace       string
	FrontendAddr         string
	ReceiverAddr         string
	ExternalURL          string
}

func main() {
	cfg := parseFlags()

	mgr, err := setupManager(cfg)
	if err != nil {
		setupLog.Error(err, "Unable to start manager")
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "Unable to create clientset")
		os.Exit(1)
	}

	sseBroker := frontend.NewSSEBroker()

	if err := setupControllers(mgr, cfg, sseBroker); err != nil {
		setupLog.Error(err, "Unable to setup controllers")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Unable to set up health check")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Unable to set up ready check")
		os.Exit(1)
	}

	if err := setupWebhooks(mgr, cfg); err != nil {
		setupLog.Error(err, "Unable to setup webhooks")
		os.Exit(1)
	}

	if err := setupHTTPServers(mgr, cfg, clientset, sseBroker); err != nil {
		setupLog.Error(err, "Unable to setup auxiliary servers")
		os.Exit(1)
	}

	setupLog.Info("Starting manager")

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Problem running manager")
		os.Exit(1)
	}
}

// parseFlags binds and parses command line flags into a Config struct.
func parseFlags() Config {
	var cfg Config

	flag.StringVar(&cfg.MetricsAddr, "metrics-bind-address", "0",
		"The address the metrics endpoint binds to. "+
			"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&cfg.ProbeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	flag.BoolVar(&cfg.EnableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&cfg.SecureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.BoolVar(&cfg.WebhookCertRotation, "webhook-cert-rotation", true,
		"Enable webhook certificate rotation if set true.")
	flag.StringVar(&cfg.WebhookCertPath, "webhook-cert-path", "/tmp/k8s-webhook-server/serving-certs",
		"The directory where webhook certificates are stored.")
	flag.BoolVar(&cfg.EnableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&cfg.WatchNamespace, "watch-namespace", "",
		"The namespace the controller will watch.")
	flag.StringVar(&cfg.FrontendAddr, "frontend-bind-address", ":8082",
		"The address the web frontend endpoint binds to.")
	flag.StringVar(&cfg.ReceiverAddr, "receiver-bind-address", "0",
		"The address the event receiver endpoint binds to.")
	flag.StringVar(&cfg.ExternalURL, "external-url", "",
		"The public base URL of the operator (e.g., https://operator.example.com). Required for webhooks.")

	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)

	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	return cfg
}

// setupManager configures and creates the controller-runtime Manager.
//
//nolint:ireturn
func setupManager(cfg Config) (manager.Manager, error) {
	var tlsOpts []func(*tls.Config)

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities.
	if !cfg.EnableHTTP2 {
		setupLog.Info("Disabling HTTP/2")

		tlsOpts = append(tlsOpts, func(c *tls.Config) {
			c.NextProtos = []string{"http/1.1"}
		})
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	metricsServerOptions := metricsserver.Options{
		BindAddress:   cfg.MetricsAddr,
		SecureServing: cfg.SecureMetrics,
		TLSOpts:       tlsOpts,
	}

	if cfg.SecureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	managerOptions := ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: cfg.ProbeAddr,
		LeaderElection:         cfg.EnableLeaderElection,
		LeaderElectionID:       "73f32edc.thegeeklab.de",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	}

	if cfg.WatchNamespace != "" {
		namespaces := watchedNamespaces(cfg.WatchNamespace)
		defaultNamespaces := make(map[string]cache.Config)

		for _, ns := range namespaces {
			defaultNamespaces[ns] = cache.Config{}
		}

		managerOptions.Cache = cache.Options{
			DefaultNamespaces: defaultNamespaces,
		}

		setupLog.Info("Listening for changes", "watchNamespaces", namespaces)
	} else {
		setupLog.Info("Listening for changes on all namespaces")
	}

	return ctrl.NewManager(ctrl.GetConfigOrDie(), managerOptions)
}

// setupControllers registers all reconcilers with the Manager.
func setupControllers(mgr manager.Manager, cfg Config, sseBroker *frontend.SSEBroker) error {
	// renovator
	if err := (&renovator.Reconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller %s: %w", renovator.ControllerName, err)
	}

	// discovery
	if err := (&discovery.Reconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller %s: %w", discovery.ControllerName, err)
	}

	// runner
	if err := (&runner.Reconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Broker: sseBroker,
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller %s: %w", runner.ControllerName, err)
	}

	// gitrepo
	if err := (&gitrepo.Reconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		ExternalURL: cfg.ExternalURL,
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller %s: %w", gitrepo.ControllerName, err)
	}

	return nil
}

// setupWebhooks handles certificate rotation and registers admission webhooks.
func setupWebhooks(mgr manager.Manager, cfg Config) error {
	if os.Getenv("ENABLE_WEBHOOKS") == "false" {
		return nil
	}

	setupFinished := make(chan struct{})

	if cfg.WebhookCertRotation {
		setupLog.Info("Setting up webhook cert rotation")

		webhooks := []rotator.WebhookInfo{
			{
				Name: webhookName,
				Type: rotator.Mutating,
			},
		}

		if err := waitForWebhooks(mgr.GetAPIReader(), webhooks); err != nil {
			return fmt.Errorf("unable to find required WebhookConfiguration %s: %w", webhookName, err)
		}

		if err := rotator.AddRotator(mgr, &rotator.CertRotator{
			SecretKey: types.NamespacedName{
				Namespace: k8s.GetNamespace(),
				Name:      webhookSecretName,
			},
			CertDir:        cfg.WebhookCertPath,
			CAName:         webhookCAName,
			CAOrganization: webhookCAOrganization,
			DNSName:        fmt.Sprintf("%s.%s.svc", webhookCertService, k8s.GetNamespace()),
			IsReady:        setupFinished,
			Webhooks:       webhooks,
		}); err != nil {
			return fmt.Errorf("unable to set up webhook cert rotation: %w", err)
		}
	} else {
		close(setupFinished)
	}

	return mgr.Add(manager.RunnableFunc((func(ctx context.Context) error {
		if cfg.WebhookCertRotation {
			setupLog.Info("Waiting for certificates to be ready before registering webhook")

			// Use select to gracefully abort if the manager context is cancelled
			select {
			case <-setupFinished:
				setupLog.Info("Certificates ready, setting up webhook")
			case <-ctx.Done():
				setupLog.Info("Manager shutting down, aborting webhook setup")

				return ctx.Err()
			}
		} else {
			setupLog.Info("Skipping cert rotation, setting up webhook")

			// Modern os.ErrNotExist check
			if _, err := os.Stat(fmt.Sprintf("%s/tls.crt", cfg.WebhookCertPath)); errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("certificate file does not exist"+
					" while certificate rotation is disabled at path %s/tls.crt: %w", cfg.WebhookCertPath, err)
			}
		}

		if err := webhookrenovatev1beta1.SetupRenovatorWebhookWithManager(mgr); err != nil {
			return fmt.Errorf("unable to create webhook %s: %w", renovator.ControllerName, err)
		}

		if err := webhookrenovatev1beta1.SetupRenovateConfigWebhookWithManager(mgr); err != nil {
			return fmt.Errorf("unable to create webhook RenovateConfig: %w", err)
		}

		if err := webhookrenovatev1beta1.SetupDiscoveryWebhookWithManager(mgr); err != nil {
			return fmt.Errorf("unable to create webhook Discovery: %w", err)
		}

		if err := webhookrenovatev1beta1.SetupRunnerWebhookWithManager(mgr); err != nil {
			return fmt.Errorf("unable to create webhook Runner: %w", err)
		}

		return nil
	})))
}

// setupHTTPServers registers the web frontend and event receiver HTTP servers.
func setupHTTPServers(
	mgr manager.Manager, cfg Config, clientset kubernetes.Interface, sseBroker *frontend.SSEBroker,
) error {
	// Setup web frontend server if enabled
	if cfg.FrontendAddr != "0" {
		frontendConfig := frontend.DefaultServerConfig()
		frontendConfig.Addr = cfg.FrontendAddr
		frontendConfig.DevMode = os.Getenv("NODE_ENV") == "development"

		frontendServer := frontend.NewServer(
			frontendConfig,
			mgr.GetClient(),
			clientset,
			sseBroker,
		)

		setupLog.Info("Adding HTTP server to manager", "server", "frontend", "addr", cfg.FrontendAddr)

		if err := mgr.Add(frontendServer); err != nil {
			return fmt.Errorf("failed to add frontend HTTP server to manager: %w", err)
		}
	}

	// Setup event receiver server if enabled
	if cfg.ReceiverAddr != "0" {
		if cfg.ExternalURL == "" {
			err := fmt.Errorf("%w: --external-url", errFlagRequired)

			setupLog.Error(err, "Missing required configuration for HTTP server",
				"server", "receiver",
				"reason", "Git providers need the public external URL to know where to send webhooks.",
			)

			return fmt.Errorf("receiver HTTP server validation failed: %w", err)
		}

		receiverConfig := receiver.DefaultServerConfig()
		receiverConfig.Addr = cfg.ReceiverAddr

		receiverServer := receiver.NewServer(
			receiverConfig,
			mgr.GetClient(),
		)

		setupLog.Info("Adding HTTP server to manager", "server", "receiver", "addr", cfg.ReceiverAddr)

		if err := mgr.Add(receiverServer); err != nil {
			return fmt.Errorf("failed to add receiver HTTP server to manager: %w", err)
		}
	}

	return nil
}

// WatchedNamespaces get the list of additional watched namespaces.
// The result is a list of namespaces specified in the WATCHED_NAMESPACE where
// each namespace is separated by comma.
func watchedNamespaces(namespaces string) []string {
	unfilteredList := strings.Split(namespaces, ",")
	result := make([]string, 0, len(unfilteredList))

	for _, elem := range unfilteredList {
		elem = strings.TrimSpace(elem)
		if len(elem) != 0 {
			result = append(result, elem)
		}
	}

	return result
}

// waitForWebhooks waits for all configured WebhookConfigurations to exist in the cluster.
func waitForWebhooks(c client.Reader, webhooks []rotator.WebhookInfo) error {
	const (
		timeout = 10 * time.Second
		sleep   = 2 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, wh := range webhooks {
		setupLog.Info("Waiting for WebhookConfiguration to become available", "name", wh.Name)

	RetryLoop:
		for {
			select {
			case <-ctx.Done():
				return fmt.Errorf("%w: %s", errWebhookTimeout, wh.Name)
			default:
				var webhookObj client.Object

				switch wh.Type {
				case rotator.Mutating:
					webhookObj = &admissionregistrationv1.MutatingWebhookConfiguration{}
				case rotator.Validating:
					webhookObj = &admissionregistrationv1.ValidatingWebhookConfiguration{}
				}

				if err := c.Get(ctx, types.NamespacedName{Name: wh.Name}, webhookObj); err == nil {
					break RetryLoop
				}

				time.Sleep(sleep)
			}
		}
	}

	return nil
}
