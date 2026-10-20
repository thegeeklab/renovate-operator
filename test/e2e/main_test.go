// Package e2e contains the end-to-end test suite for the operator. It runs
// against an isolated Kind cluster that is created, populated with the locally
// built operator image and destroyed as part of the test lifecycle.
package e2e

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"testing"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/test/utils"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support"
	"sigs.k8s.io/e2e-framework/support/kind"
)

const (
	// defaultProjectImage is used when the E2E_IMAGE environment variable is
	// not set. It must not use the "latest" tag so that the loaded image is
	// preferred over a pull from the registry.
	defaultProjectImage = "docker.io/thegeeklab/renovate-operator:e2e"
	// defaultStubImage is used when the E2E_STUB_IMAGE environment variable is
	// not set. It is the no-op image that stands in for the Renovate container
	// in the job-execution e2e tests.
	defaultStubImage = "docker.io/thegeeklab/renovate-operator-stub:e2e"
)

var (
	testEnv       env.Environment
	testNamespace string
	projectImage  string
	stubImage     string
	clusterName   string
	finishFuncs   []env.Func
	testCfg       *envconf.Config
	cleanupOnce   sync.Once
)

func init() {
	utilruntime.Must(renovatev1beta1.AddToScheme(clientgoscheme.Scheme))
}

// runCleanup executes all finish functions to clean up test resources.
// This is called both by the e2e framework on normal completion and by
// the signal handler on interrupt.
func runCleanup() {
	cleanupOnce.Do(func() {
		if testCfg == nil {
			return
		}

		ctx := context.Background()

		for _, fn := range finishFuncs {
			if _, err := fn(ctx, testCfg); err != nil {
				os.Stderr.WriteString("cleanup error: " + err.Error() + "\n")
			}
		}
	})
}

// TestMain assembles the test environment: it creates the Kind cluster, builds
// and loads the operator image, deploys the operator and registers the
// teardown steps that run after all tests have finished.
func TestMain(m *testing.M) {
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		panic(err)
	}

	testCfg = cfg
	testEnv = env.NewWithConfig(cfg)

	// skipTeardown keeps the Kind cluster and namespace around after the test
	// run for debugging, instead of running the registered finish steps.
	skipTeardown := os.Getenv("E2E_SKIP_TEARDOWN") == "true"

	if skipTeardown {
		// A stable name makes the leftover cluster easy to reference and clean
		// up manually while debugging.
		clusterName = "renovate-operator"
	} else {
		clusterName = envconf.RandomName("renovate-operator", 16)
	}

	testNamespace = envconf.RandomName("renovate-e2e", 16)

	projectImage = os.Getenv("E2E_IMAGE")
	if projectImage == "" {
		projectImage = defaultProjectImage
	}

	stubImage = os.Getenv("E2E_STUB_IMAGE")
	if stubImage == "" {
		stubImage = defaultStubImage
	}

	// kindNodeImage is set by make test-e2e from KIND_NODE_IMAGE_VERSION; when
	// unset the framework falls back to the Kind CLI's default node image.
	kindNodeImage := os.Getenv("KIND_NODE_IMAGE")

	setup := []env.Func{}
	finishFuncs = []env.Func{}

	if cfg.KubeconfigFile() == "" {
		clusterOpts := []support.ClusterOpts{}
		if kindNodeImage != "" {
			clusterOpts = append(clusterOpts, kind.WithImage(kindNodeImage))
		}

		setup = append(setup,
			envfuncs.CreateClusterWithOpts(kind.NewProvider(), clusterName, clusterOpts...),
			envfuncs.CreateNamespace(testNamespace),
			utils.BuildImage(projectImage),
			utils.BuildStubImage(stubImage),
			envfuncs.LoadImageToCluster(clusterName, projectImage),
			envfuncs.LoadImageToCluster(clusterName, stubImage),
			utils.DeployOperator(projectImage),
		)
		if !skipTeardown {
			finishFuncs = append(finishFuncs,
				utils.UndeployOperator(),
				envfuncs.DeleteNamespace(testNamespace),
				envfuncs.DestroyCluster(clusterName),
			)
		}
	} else {
		setup = append(setup,
			envfuncs.CreateNamespace(testNamespace),
			utils.DeployOperator(projectImage),
		)
		if !skipTeardown {
			finishFuncs = append(finishFuncs,
				utils.UndeployOperator(),
				envfuncs.DeleteNamespace(testNamespace),
			)
		}
	}

	testEnv.Setup(setup...)

	if skipTeardown {
		fmt.Fprintf(os.Stderr, "E2E_SKIP_TEARDOWN=true: skipping teardown; namespace %q left in place\n", testNamespace)

		if cfg.KubeconfigFile() == "" {
			fmt.Fprintf(os.Stderr, "clean up the cluster with: kind delete cluster --name %s\n", clusterName)
		}
	} else {
		testEnv.Finish(finishFuncs...)
	}

	// Set up signal handler to ensure cleanup on interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	go func() {
		<-sigCh
		os.Stderr.WriteString("\nInterrupted, cleaning up...\n")
		runCleanup()
		os.Exit(1)
	}()

	os.Exit(testEnv.Run(m))
}
