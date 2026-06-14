# renovate-operator

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.1.0](https://img.shields.io/badge/AppVersion-0.1.0-informational?style=flat-square)

A Helm chart to distribute the project renovate-operator

## Maintainers

| Name                          | Email                | Url                                               |
| ----------------------------- | -------------------- | ------------------------------------------------- |
| Renovate Operator Maintainers | <mail@thegeeklab.de> | <https://github.com/thegeeklab/renovate-operator> |

## Values

| Key                                              | Type   | Default                                                                                  | Description                                                                           |
| ------------------------------------------------ | ------ | ---------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------- |
| certManager.enable                               | bool   | `true`                                                                                   | Enable cert-manager integration for webhook and metrics certificates                  |
| crd.enable                                       | bool   | `true`                                                                                   | Install CRDs with the chart                                                           |
| crd.keep                                         | bool   | `true`                                                                                   | Keep CRDs when uninstalling the chart                                                 |
| fullnameOverride                                 | string | `""`                                                                                     | String to fully override chart.fullname template                                      |
| manager.affinity                                 | object | `{}`                                                                                     | Node affinity rules for the manager pod                                               |
| manager.args                                     | list   | `["--leader-elect","--frontend-bind-address=:8082"]`                                     | Arguments passed to the manager container                                             |
| manager.env                                      | list   | `[{"name":"POD_NAMESPACE","valueFrom":{"fieldRef":{"fieldPath":"metadata.namespace"}}}]` | Environment variables for the manager container                                       |
| manager.image.pullPolicy                         | string | `"IfNotPresent"`                                                                         | Manager container image pull policy                                                   |
| manager.image.repository                         | string | `"docker.io/thegeeklab/renovate-operator"`                                               | Manager container image repository                                                    |
| manager.image.tag                                | string | `"latest"`                                                                               | Manager container image tag                                                           |
| manager.imagePullSecrets                         | list   | `[]`                                                                                     | Image pull secrets for the manager pod                                                |
| manager.nodeSelector                             | object | `{}`                                                                                     | Node selector for the manager pod                                                     |
| manager.podSecurityContext.runAsNonRoot          | bool   | `true`                                                                                   | Ensure the pod runs as a non-root user                                                |
| manager.podSecurityContext.seccompProfile.type   | string | `"RuntimeDefault"`                                                                       | Type of seccomp profile                                                               |
| manager.replicas                                 | int    | `1`                                                                                      | Number of replicas for the manager deployment                                         |
| manager.resources.limits.cpu                     | string | `"500m"`                                                                                 | CPU limit for the manager container                                                   |
| manager.resources.limits.memory                  | string | `"128Mi"`                                                                                | Memory limit for the manager container                                                |
| manager.resources.requests.cpu                   | string | `"10m"`                                                                                  | CPU request for the manager container                                                 |
| manager.resources.requests.memory                | string | `"64Mi"`                                                                                 | Memory request for the manager container                                              |
| manager.securityContext.allowPrivilegeEscalation | bool   | `false`                                                                                  | Prevent privilege escalation                                                          |
| manager.securityContext.capabilities.drop[0]     | string | `"ALL"`                                                                                  |                                                                                       |
| manager.tolerations                              | list   | `[]`                                                                                     | Tolerations for the manager pod                                                       |
| metrics.enable                                   | bool   | `true`                                                                                   | Enable the /metrics endpoint with RBAC protection                                     |
| metrics.port                                     | int    | `8443`                                                                                   | Metrics server port                                                                   |
| nameOverride                                     | string | `""`                                                                                     | String to partially override chart.fullname template (will maintain the release name) |
| prometheus.enable                                | bool   | `false`                                                                                  | Enable ServiceMonitor creation (requires prometheus-operator)                         |
| rbacHelpers.enable                               | bool   | `false`                                                                                  | Install convenience admin/editor/viewer roles for CRDs                                |
| webhook.enable                                   | bool   | `true`                                                                                   | Enable the validating/mutating webhook server                                         |
| webhook.port                                     | int    | `9443`                                                                                   | Webhook server port                                                                   |
