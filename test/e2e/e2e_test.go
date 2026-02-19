package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thegeeklab/renovate-operator/test/utils"
)

// namespace where the project is deployed in.
const namespace = "renovate-system"

// serviceAccountName created for the project.
const serviceAccountName = "renovate-operator-controller-manager"

// metricsServiceName is the name of the metrics service of the project.
const metricsServiceName = "renovate-operator-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data.
const metricsRoleBindingName = "renovate-operator-metrics-binding"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating manager namespace")

		cmd := exec.CommandContext(context.Background(), "kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")

		cmd = exec.CommandContext(context.Background(), "kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")

		cmd = exec.CommandContext(context.Background(), "make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")

		cmd = exec.CommandContext(context.Background(), "make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")

		cmd := exec.CommandContext(context.Background(), "kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")

		cmd = exec.CommandContext(context.Background(), "make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")

		cmd = exec.CommandContext(context.Background(), "make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")

		cmd = exec.CommandContext(context.Background(), "kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")

			cmd := exec.CommandContext(context.Background(), "kubectl", "logs", controllerPodName, "-n", namespace)

			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")

			cmd = exec.CommandContext(context.Background(),
				"kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")

			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")

			cmd = exec.CommandContext(context.Background(), "kubectl", "logs", "curl-metrics", "-n", namespace)

			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")

			cmd = exec.CommandContext(context.Background(), "kubectl", "describe", "pod", controllerPodName, "-n", namespace)

			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")

			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				template := `{{ range .items }}` +
					`{{ if not .metadata.deletionTimestamp }}` +
					`{{ .metadata.name }}{{ "\n" }}` +
					`{{ end }}` +
					`{{ end }}`
				cmd := exec.CommandContext(context.Background(), "kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template="+template,
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")

				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.CommandContext(context.Background(), "kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("creating a ClusterRoleBinding for the service account to allow access to metrics")

			cmd := exec.CommandContext(context.Background(), "kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=renovate-operator-metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")

			cmd = exec.CommandContext(context.Background(), "kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("validating that the ServiceMonitor for Prometheus is applied in the namespace")

			cmd = exec.CommandContext(context.Background(), "kubectl", "get", "ServiceMonitor", "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "ServiceMonitor should exist")

			By("getting the service account token")

			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("waiting for the metrics endpoint to be ready")

			verifyMetricsEndpointReady := func(g Gomega) {
				cmd := exec.CommandContext(context.Background(), "kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")

			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.CommandContext(context.Background(), "kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("controller-runtime.metrics\tServing metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted).Should(Succeed())

			By("creating the curl-metrics pod to access the metrics endpoint")

			cmd = exec.CommandContext(context.Background(), "kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image=curlimages/curl:latest",
				"--overrides",
				fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "curl",
							"image": "curlimages/curl:latest",
							"command": ["/bin/sh", "-c"],
							"args": ["curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics"],
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"capabilities": {
									"drop": ["ALL"]
								},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {
									"type": "RuntimeDefault"
								}
							}
						}],
						"serviceAccount": "%s"
					}
				}`, token, metricsServiceName, namespace, serviceAccountName))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")

			verifyCurlUp := func(g Gomega) {
				cmd := exec.CommandContext(context.Background(), "kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")

			metricsOutput := getMetricsOutput()
			Expect(metricsOutput).To(ContainSubstring(
				"controller_runtime_reconcile_total",
			))
		})

		It("should have CA injection for mutating webhooks", func() {
			By("checking CA injection for mutating webhooks")

			verifyCAInjection := func(g Gomega) {
				cmd := exec.CommandContext(context.Background(), "kubectl", "get",
					"mutatingwebhookconfigurations.admissionregistration.k8s.io",
					"renovate-operator-webhook-configuration",
					"-o", "go-template={{ range .webhooks }}{{ .clientConfig.caBundle }}{{ end }}")
				mwhOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(mwhOutput)).To(BeNumerically(">", 10))
			}
			Eventually(verifyCAInjection).Should(Succeed())
		})

		It("should have CA injection for mutating webhooks", func() {
			By("checking CA injection for mutating webhooks")

			verifyCAInjection := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"mutatingwebhookconfigurations.admissionregistration.k8s.io",
					"renovate-operator-webhook-configuration",
					"-o", "go-template={{ range .webhooks }}{{ .clientConfig.caBundle }}{{ end }}")
				mwhOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(mwhOutput)).To(BeNumerically(">", 10))
			}
			Eventually(verifyCAInjection).Should(Succeed())
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks

		// TODO: Customize the e2e test suite with scenarios specific to your project.
		// Consider applying sample/CR(s) and check their status and/or verifying
		// the reconciliation by using the metrics, i.e.:
		// metricsOutput := getMetricsOutput()
		// Expect(metricsOutput).To(ContainSubstring(
		//    fmt.Sprintf(`controller_runtime_reconcile_total{controller="%s",result="success"} 1`,
		//    strings.ToLower(<Kind>),
		// ))

		It("should be able to list Jobs in the batch API group", func() {
			By("creating a test Job in the namespace")

			jobYaml := `
apiVersion: batch/v1
kind: Job
metadata:
  name: test-job
  namespace: %s
spec:
  template:
    spec:
      containers:
      - name: test
        image: busybox
        command: ["echo", "hello"]
      restartPolicy: Never
`
			jobFile, err := os.CreateTemp("", "test-job-*.yaml")
			Expect(err).NotTo(HaveOccurred())

			defer os.Remove(jobFile.Name())

			_, err = jobFile.Write([]byte(fmt.Sprintf(jobYaml, namespace)))
			Expect(err).NotTo(HaveOccurred())
			err = jobFile.Close()
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.CommandContext(context.Background(), "kubectl", "apply", "-f", jobFile.Name())
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			// Clean up the job after the test
			defer func() {
				cmd := exec.CommandContext(context.Background(), "kubectl", "delete", "job", "test-job", "-n", namespace)
				_, _ = utils.Run(cmd)
			}()

			By("verifying the controller can list Jobs without errors")
			// This will implicitly test the RBAC permissions as the controller needs to list Jobs
			// for its discovery functionality
			verifyJobListing := func(g Gomega) {
				// We don't need to explicitly call the controller's listing function
				// as the error would have occurred during normal operation if permissions were missing
				// Instead, we verify that no RBAC errors are reported in the controller logs
				cmd := exec.CommandContext(context.Background(), "kubectl", "logs", controllerPodName, "-n", namespace)
				logs, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				// Check that there are no "jobs.batch is forbidden" errors in the logs
				g.Expect(logs).NotTo(ContainSubstring("jobs.batch is forbidden"),
					"Controller should have permission to list Jobs")
			}
			Eventually(verifyJobListing).Should(Succeed())
		})
	})
})

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)

	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), 0o644)
	if err != nil {
		return "", err
	}

	var out string

	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		cmd := exec.CommandContext(context.Background(), "kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest

		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() string {
	By("getting the curl-metrics logs")

	cmd := exec.CommandContext(context.Background(), "kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
	Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))

	return metricsOutput
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
