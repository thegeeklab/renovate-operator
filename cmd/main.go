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

	"github.com/go-logr/logr"
	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/controller/authprovider"
	"github.com/thegeeklab/renovate-operator/internal/controller/discovery"
	"github.com/thegeeklab/renovate-operator/internal/controller/gitrepo"
	"github.com/thegeeklab/renovate-operator/internal/controller/renovator"
	"github.com/thegeeklab/renovate-operator/internal/controller/runner"
	"github.com/thegeeklab/renovate-operator/internal/frontend"
	"github.com/thegeeklab/renovate-operator/internal/frontend/auth"
	"github.com/thegeeklab/renovate-operator/internal/logreader"
	"github.com/thegeeklab/renovate-operator/internal/metrics"
	"github.com/thegeeklab/renovate-operator/internal/receiver"
	"github.com/thegeeklab/renovate-operator/internal/receiver/gitea"
	"github.com/thegeeklab/renovate-operator/internal/receiver/github"
	"github.com/thegeeklab/renovate-operator/internal/receiver/gitlab"
	"github.com/thegeeklab/renovate-operator/internal/telemetry"
	webhookrenovatev1beta1 "github.com/thegeeklab/renovate-operator/internal/webhook/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	// +kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()

	errWebhookTimeout = errors.New("timeout waiting for webhook")
	errFlagRequired   = errors.New("missing required flag")
	errInvalidDNSName = errors.New("invalid DNS name")

	version  = "unknown"
	setupLog logr.Logger
)

const (
	webhookCAName         = "renovate-operator-ca"
	webhookCAOrganization = "renovate-operator"

	// cert-controller names its controller "cert-rotator" by default, and
	// controller-runtime rejects duplicate controller names. Give each rotator
	// its own name so the metrics and webhook certificates can both be rotated.
	metricsCertRotatorName = "metrics-cert-rotator"
	webhookCertRotatorName = "webhook-cert-rotator"

	otelShutdownTimeout = 5 * time.Second
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
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch;update

//nolint:wsl
func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(renovatev1beta1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// Config holds all command-line configuration for the operator.
type Config struct {
	MetricsAddr           string
	EnableLeaderElection  bool
	ProbeAddr             string
	SecureMetrics         bool
	MetricsCertRotation   bool
	MetricsCertPath       string
	MetricsSecretName     string
	MetricsServiceName    string
	WebhookCertRotation   bool
	WebhookCertPath       string
	WebhookName           string
	WebhookSecretName     string
	WebhookServiceName    string
	EnableHTTP2           bool
	WatchNamespace        string
	FrontendAddr          string
	ReceiverAddr          string
	ExternalURL           string
	SecureCookies         bool
	MetricsCardinalityCap int
}

func main() {
	if err := run(); err != nil {
		setupLog.Error(err, "Fatal error")
		os.Exit(1)
	}
}

func run() error {
	cfg := parseFlags()
	setupLog = ctrl.Log.WithName("setup")

	mgr, err := setupManager(cfg)
	if err != nil {
		return fmt.Errorf("unable to start manager: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("unable to create clientset: %w", err)
	}

	sseBroker := frontend.NewSSEBroker()
	authManager := auth.NewManager(cfg.SecureCookies)

	metricsRecorder := metrics.New(ctrlmetrics.Registry, ctrlmetrics.Registry, cfg.MetricsCardinalityCap)

	logReader := logreader.NewKubernetesReader(clientset)

	// Setup OpenTelemetry Prometheus bridge if OTEL_EXPORTER_OTLP_ENDPOINT is set
	otelShutdown, err := telemetry.SetupPrometheusBridge(context.Background(), ctrlmetrics.Registry, version)
	if err != nil {
		setupLog.Error(err, "Failed to setup OpenTelemetry bridge, continuing without OTLP export")
	}

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), otelShutdownTimeout)
		defer cancel()

		if err := otelShutdown(shutdownCtx); err != nil {
			setupLog.Error(err, "Failed to shutdown OpenTelemetry provider")
		}
	}()

	if err == nil && os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		setupLog.Info("OpenTelemetry Prometheus bridge enabled")
	}

	if err := setupControllers(mgr, cfg, sseBroker, authManager, metricsRecorder, logReader); err != nil {
		return fmt.Errorf("unable to setup controllers: %w", err)
	}

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}

	if err := setupMetricsCertRotation(mgr, cfg); err != nil {
		return fmt.Errorf("unable to setup metrics certificate rotation: %w", err)
	}

	if err := setupWebhooks(mgr, cfg); err != nil {
		return fmt.Errorf("unable to setup webhooks: %w", err)
	}

	if err := setupHTTPServers(mgr, cfg, clientset, sseBroker, authManager, metricsRecorder, logReader); err != nil {
		return fmt.Errorf("unable to setup auxiliary servers: %w", err)
	}

	setupLog.Info("Starting manager")

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		return fmt.Errorf("problem running manager: %w", err)
	}

	return nil
}

// parseFlags binds and parses command line flags into a Config struct.
//
//nolint:mnd
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
	flag.BoolVar(&cfg.MetricsCertRotation, "metrics-cert-rotation", true,
		"Enable metrics server certificate rotation if set true.")
	flag.StringVar(&cfg.MetricsCertPath, "metrics-cert-path", "/tmp/k8s-metrics-server/serving-certs",
		"The directory where metrics server certificates are stored.")
	flag.StringVar(&cfg.MetricsSecretName, "metrics-cert-secret-name", "renovate-operator-metrics-server-cert",
		"The name of the Secret containing metrics server certificates.")
	flag.StringVar(&cfg.MetricsServiceName, "metrics-service-name", "renovate-operator-metrics-service",
		"The name of the metrics Service (used for certificate SAN).")
	flag.BoolVar(&cfg.WebhookCertRotation, "webhook-cert-rotation", true,
		"Enable webhook certificate rotation if set true.")
	flag.StringVar(&cfg.WebhookCertPath, "webhook-cert-path", "/tmp/k8s-webhook-server/serving-certs",
		"The directory where webhook certificates are stored.")
	flag.StringVar(&cfg.WebhookName, "webhook-name", "renovate-operator-webhook-configuration",
		"The name of the MutatingWebhookConfiguration (used for cert patching).")
	flag.StringVar(&cfg.WebhookSecretName, "webhook-cert-secret-name", "renovate-operator-webhook-server-cert",
		"The name of the Secret containing webhook certificates.")
	flag.StringVar(&cfg.WebhookServiceName, "webhook-service-name", "renovate-operator-webhook-service",
		"The name of the webhook Service (used for certificate SAN).")
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
	flag.BoolVar(&cfg.SecureCookies, "secure-cookies", true,
		"Force Secure attribute on auth cookies. Set to false for localhost-only development.")
	flag.IntVar(&cfg.MetricsCardinalityCap, "metrics-cardinality-cap", 5000,
		"Maximum number of unique label combinations tracked before series are dropped.")

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
		CertDir: cfg.WebhookCertPath,
		TLSOpts: tlsOpts,
	})

	metricsServerOptions := metricsserver.Options{
		BindAddress:   cfg.MetricsAddr,
		SecureServing: cfg.SecureMetrics,
		CertDir:       cfg.MetricsCertPath,
		CertName:      "tls.crt",
		KeyName:       "tls.key",
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
func setupControllers(
	mgr manager.Manager,
	cfg Config,
	sseBroker *frontend.SSEBroker,
	authManager *auth.Manager,
	metricsRecorder metrics.Recorder,
	logReader logreader.Reader,
) error {
	if os.Getenv("ENABLE_CONTROLLERS") == "false" {
		return nil
	}

	if err := (&renovator.Reconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller %s: %w", renovator.ControllerName, err)
	}

	if err := (&discovery.Reconciler{
		Client:  mgr.GetClient(),
		Scheme:  mgr.GetScheme(),
		Metrics: metricsRecorder,
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller %s: %w", discovery.ControllerName, err)
	}

	if err := (&runner.Reconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		Broker:    sseBroker,
		Metrics:   metricsRecorder,
		LogReader: logReader,
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller %s: %w", runner.ControllerName, err)
	}

	if err := (&gitrepo.Reconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		ExternalURL: cfg.ExternalURL,
		Broker:      sseBroker,
		Metrics:     metricsRecorder,
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller %s: %w", gitrepo.ControllerName, err)
	}

	if err := (&authprovider.Reconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		AuthManager: authManager,
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller %s: %w", authprovider.ControllerName, err)
	}

	return nil
}

// setupMetricsCertRotation configures certificate rotation for the metrics server.
// The metrics server loads certificates from CertDir dynamically, so the rotator
// can be started fire-and-forget without blocking startup.
func setupMetricsCertRotation(mgr manager.Manager, cfg Config) error {
	if cfg.MetricsAddr == "0" || !cfg.SecureMetrics {
		return nil
	}

	if err := validateDNSName(cfg.MetricsServiceName, "metrics-service-name"); err != nil {
		return err
	}

	if !cfg.MetricsCertRotation {
		setupLog.Info("Skipping metrics cert rotation")

		return verifyCertFiles(cfg.MetricsCertPath, "metrics")
	}

	setupLog.Info("Setting up metrics cert rotation")

	if err := rotator.AddRotator(mgr, &rotator.CertRotator{
		SecretKey: types.NamespacedName{
			Namespace: k8s.GetNamespace(),
			Name:      cfg.MetricsSecretName,
		},
		CertDir:        cfg.MetricsCertPath,
		CAName:         webhookCAName,
		CAOrganization: webhookCAOrganization,
		DNSName:        fmt.Sprintf("%s.%s.svc", cfg.MetricsServiceName, k8s.GetNamespace()),
		ControllerName: metricsCertRotatorName,
		// The rotator closes IsReady unconditionally once the CA is injected, so a
		// nil channel panics. Nothing waits on this one: the metrics server picks
		// up the certificate from CertDir via its own certwatcher.
		IsReady:  make(chan struct{}),
		Webhooks: []rotator.WebhookInfo{},
	}); err != nil {
		return fmt.Errorf("unable to set up metrics cert rotation: %w", err)
	}

	return nil
}

// setupWebhooks configures webhook certificate rotation if enabled and registers
// webhook handlers once the webhook certificate is ready.
func setupWebhooks(mgr manager.Manager, cfg Config) error {
	if os.Getenv("ENABLE_WEBHOOKS") == "false" {
		return nil
	}

	if err := validateDNSName(cfg.WebhookServiceName, "webhook-service-name"); err != nil {
		return err
	}

	var webhookReady chan struct{}

	if cfg.WebhookCertRotation {
		setupLog.Info("Setting up webhook cert rotation")

		webhooks := []rotator.WebhookInfo{
			{
				Name: cfg.WebhookName,
				Type: rotator.Mutating,
			},
			{
				Name: cfg.WebhookName,
				Type: rotator.Validating,
			},
		}

		if err := waitForWebhooks(mgr.GetAPIReader(), webhooks); err != nil {
			return fmt.Errorf("unable to find required WebhookConfiguration %s: %w", cfg.WebhookName, err)
		}

		webhookReady = make(chan struct{})

		if err := rotator.AddRotator(mgr, &rotator.CertRotator{
			SecretKey: types.NamespacedName{
				Namespace: k8s.GetNamespace(),
				Name:      cfg.WebhookSecretName,
			},
			CertDir:        cfg.WebhookCertPath,
			CAName:         webhookCAName,
			CAOrganization: webhookCAOrganization,
			DNSName:        fmt.Sprintf("%s.%s.svc", cfg.WebhookServiceName, k8s.GetNamespace()),
			ControllerName: webhookCertRotatorName,
			IsReady:        webhookReady,
			Webhooks:       webhooks,
		}); err != nil {
			return fmt.Errorf("unable to set up webhook cert rotation: %w", err)
		}
	}

	return mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		if cfg.WebhookCertRotation {
			setupLog.Info("Waiting for webhook certificate to be ready before registering handlers")

			select {
			case <-webhookReady:
				setupLog.Info("Webhook certificate ready, registering handlers")
			case <-ctx.Done():
				setupLog.Info("Manager shutting down, aborting webhook setup")

				return ctx.Err()
			}
		} else {
			setupLog.Info("Skipping cert rotation, setting up webhook")

			if err := verifyCertFiles(cfg.WebhookCertPath, "webhook"); err != nil {
				return err
			}
		}

		setupLog.Info("Registering webhook handlers")

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
	}))
}

// setupHTTPServers registers the web frontend and event receiver HTTP servers.
func setupHTTPServers(
	mgr manager.Manager,
	cfg Config,
	clientset kubernetes.Interface,
	sseBroker *frontend.SSEBroker,
	authManager *auth.Manager,
	metricsRecorder metrics.Recorder,
	logReader logreader.Reader,
) error {
	if cfg.FrontendAddr != "0" {
		frontendConfig := frontend.DefaultServerConfig()
		frontendConfig.Addr = cfg.FrontendAddr
		frontendConfig.DevMode = os.Getenv("NODE_ENV") == "development"
		frontendConfig.SecureCookies = cfg.SecureCookies

		frontendServer := frontend.NewServer(
			frontendConfig,
			mgr.GetClient(),
			clientset,
			sseBroker,
			authManager,
			logReader,
		)

		setupLog.Info("Adding HTTP server to manager", "server", "frontend", "addr", cfg.FrontendAddr)

		if err := mgr.Add(frontendServer); err != nil {
			return fmt.Errorf("failed to add frontend HTTP server to manager: %w", err)
		}
	}

	if cfg.ReceiverAddr != "0" {
		if cfg.ExternalURL == "" {
			err := fmt.Errorf("%w: --external-url", errFlagRequired)

			setupLog.Error(
				err, "Missing required configuration for HTTP server",
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
			buildReceiverFactory(),
			metricsRecorder,
		)

		setupLog.Info("Adding HTTP server to manager", "server", "receiver", "addr", cfg.ReceiverAddr)

		if err := mgr.Add(receiverServer); err != nil {
			return fmt.Errorf("failed to add receiver HTTP server to manager: %w", err)
		}
	}

	return nil
}

// watchedNamespaces get the list of additional watched namespaces.
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

// buildReceiverFactory returns a factory function that creates the appropriate
// webhook Receiver implementation based on the platform type.
func buildReceiverFactory() receiver.ReceiverFactory {
	return func(platformType renovatev1beta1.PlatformType) receiver.Receiver {
		switch platformType {
		case renovatev1beta1.PlatformType_GITEA:
			return gitea.NewReceiver()
		case renovatev1beta1.PlatformType_GITHUB:
			return github.NewReceiver()
		case renovatev1beta1.PlatformType_GITLAB:
			return gitlab.NewReceiver()
		default:
			return nil
		}
	}
}

// verifyCertFiles checks that the required TLS certificate and key files exist at the given path.
func verifyCertFiles(certPath, label string) error {
	crtPath := fmt.Sprintf("%s/tls.crt", certPath)
	keyPath := fmt.Sprintf("%s/tls.key", certPath)

	if _, err := os.Stat(crtPath); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%s certificate file does not exist at path %s: %w", label, crtPath, err)
	}

	if _, err := os.Stat(keyPath); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%s certificate key file does not exist at path %s: %w", label, keyPath, err)
	}

	return nil
}

// validateDNSName validates that a name is a valid DNS subdomain.
func validateDNSName(name, flagName string) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("%w for flag %s: %q: %s", errInvalidDNSName, flagName, name, strings.Join(errs, ", "))
	}

	return nil
}
