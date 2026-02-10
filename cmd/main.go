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
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/controller/discovery"
	"github.com/thegeeklab/renovate-operator/internal/controller/renovator"
	runner "github.com/thegeeklab/renovate-operator/internal/controller/runner"
	webhookrenovatev1beta1 "github.com/thegeeklab/renovate-operator/internal/webhook/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
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
// +kubebuilder:rbac:groups="coordination.k8s.io",namespace=system,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",namespace=system,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",namespace=system,resources=secrets,verbs=create;delete;get;update;patch;list;watch

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

//nolint:gocognit,maintidx
func main() {
	var (
		metricsAddr          string
		enableLeaderElection bool
		probeAddr            string
		secureMetrics        bool
		webhookCertRotation  bool
		webhookCertPath      string
		enableHTTP2          bool
		tlsOpts              []func(*tls.Config)
		watchNamespace       string
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.BoolVar(&webhookCertRotation, "webhook-cert-rotation", true, "Enable webhook certificate rotation if set true.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "/tmp/k8s-webhook-server/serving-certs",
		"The directory where webhook certificates are stored.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&watchNamespace, "watch-namespace", "", "The namespace the controller will watch.")

	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)

	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities.
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("Disabling HTTP/2")

		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	managerOptions := ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
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

	if watchNamespace != "" {
		namespaces := watchedNamespaces(watchNamespace)
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

	kubeConfig, err := ctrl.GetConfig()
	if err != nil {
		setupLog.Error(err, "Unable to get client config")
		os.Exit(1)
	}

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		setupLog.Error(err, "Unable to create Kubernetes client")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(kubeConfig, managerOptions)
	if err != nil {
		setupLog.Error(err, "Unable to start manager")
		os.Exit(1)
	}

	setupFinished := make(chan struct{})

	if webhookCertRotation {
		setupLog.Info("Setting up webhook cert rotation")

		webhooks := []rotator.WebhookInfo{
			{
				Name: webhookName,
				Type: rotator.Mutating,
			},
		}

		if err := waitForWebhooks(kubeClient, webhooks); err != nil {
			setupLog.Error(err, "Unable to find required WebhookConfiguration", "webhookName", webhookName)
			os.Exit(1)
		}

		if err := rotator.AddRotator(mgr, &rotator.CertRotator{
			SecretKey: types.NamespacedName{
				Namespace: k8s.GetNamespace(),
				Name:      webhookSecretName,
			},
			CertDir:        webhookCertPath,
			CAName:         webhookCAName,
			CAOrganization: webhookCAOrganization,
			DNSName:        fmt.Sprintf("%s.%s.svc", webhookCertService, k8s.GetNamespace()),
			IsReady:        setupFinished,
			Webhooks:       webhooks,
		}); err != nil {
			setupLog.Error(err, "Unable to set up webhook cert rotation")
			os.Exit(1)
		}
	} else {
		close(setupFinished)
	}

	// renovator
	if err = (&renovator.Reconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Unable to create controller", "controller", renovator.ControllerName)
		os.Exit(1)
	}

	// discovery
	if err = (&discovery.Reconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Unable to create controller", "controller", discovery.ControllerName)
		os.Exit(1)
	}

	// runner
	if err = (&runner.Reconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Unable to create controller", "controller", runner.ControllerName)
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

	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err := mgr.Add(manager.RunnableFunc((func(ctx context.Context) error {
			if webhookCertRotation {
				setupLog.Info("Waiting for certificates to be ready before registering webhook")
				<-setupFinished
				setupLog.Info("Certificates ready, setting up webhook")
			} else {
				setupLog.Info("Skipping cert rotation, setting up webhook")

				if _, err := os.Stat(fmt.Sprintf("%s/tls.crt", webhookCertPath)); os.IsNotExist(err) {
					setupLog.Error(err,
						"Certificate file does not exist while certificate rotation is disabled",
						"path", fmt.Sprintf("%s/tls.crt", webhookCertPath))
					os.Exit(1)
				}
			}

			if err = webhookrenovatev1beta1.SetupRenovatorWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "Unable to create webhook", "webhook", renovator.ControllerName)
				os.Exit(1)
			}

			if err = webhookrenovatev1beta1.SetupRenovateConfigWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "Unable to create webhook", "webhook", "RenovateConfig")
				os.Exit(1)
			}

			if err = webhookrenovatev1beta1.SetupDiscoveryWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "Unable to create webhook", "webhook", "Discovery")
				os.Exit(1)
			}

			if err = webhookrenovatev1beta1.SetupRunnerWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "Unable to create webhook", "webhook", "Runner")
				os.Exit(1)
			}

			return nil
		}))); err != nil {
			setupLog.Error(err, "Unable to register webhook setup hook")
			os.Exit(1)
		}
	}

	setupLog.Info("Starting manager")

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Problem running manager")
		os.Exit(1)
	}
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
func waitForWebhooks(clientset *kubernetes.Clientset, webhooks []rotator.WebhookInfo) error {
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
				var err error

				switch wh.Type {
				case rotator.Mutating:
					_, err = clientset.
						AdmissionregistrationV1().
						MutatingWebhookConfigurations().
						Get(ctx, wh.Name, metav1.GetOptions{})
				case rotator.Validating:
					_, err = clientset.
						AdmissionregistrationV1().
						ValidatingWebhookConfigurations().
						Get(ctx, wh.Name, metav1.GetOptions{})
				}

				if err == nil {
					break RetryLoop
				}

				time.Sleep(sleep)
			}
		}
	}

	return nil
}
