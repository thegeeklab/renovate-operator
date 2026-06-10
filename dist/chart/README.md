# renovate-operator

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.1.0](https://img.shields.io/badge/AppVersion-0.1.0-informational?style=flat-square)

A Helm chart to distribute the project renovate-operator

## Values

| Key                                              | Type   | Default                                    | Description |
| ------------------------------------------------ | ------ | ------------------------------------------ | ----------- |
| certManager.enable                               | bool   | `true`                                     |             |
| crd.enable                                       | bool   | `true`                                     |             |
| crd.keep                                         | bool   | `true`                                     |             |
| manager.affinity                                 | object | `{}`                                       |             |
| manager.args[0]                                  | string | `"--leader-elect"`                         |             |
| manager.args[1]                                  | string | `"--frontend-bind-address=:8082"`          |             |
| manager.env[0].name                              | string | `"POD_NAMESPACE"`                          |             |
| manager.env[0].valueFrom.fieldRef.fieldPath      | string | `"metadata.namespace"`                     |             |
| manager.image.pullPolicy                         | string | `"IfNotPresent"`                           |             |
| manager.image.repository                         | string | `"docker.io/thegeeklab/renovate-operator"` |             |
| manager.image.tag                                | string | `"latest"`                                 |             |
| manager.imagePullSecrets                         | list   | `[]`                                       |             |
| manager.nodeSelector                             | object | `{}`                                       |             |
| manager.podSecurityContext.runAsNonRoot          | bool   | `true`                                     |             |
| manager.podSecurityContext.seccompProfile.type   | string | `"RuntimeDefault"`                         |             |
| manager.replicas                                 | int    | `1`                                        |             |
| manager.resources.limits.cpu                     | string | `"500m"`                                   |             |
| manager.resources.limits.memory                  | string | `"128Mi"`                                  |             |
| manager.resources.requests.cpu                   | string | `"10m"`                                    |             |
| manager.resources.requests.memory                | string | `"64Mi"`                                   |             |
| manager.securityContext.allowPrivilegeEscalation | bool   | `false`                                    |             |
| manager.securityContext.capabilities.drop[0]     | string | `"ALL"`                                    |             |
| manager.tolerations                              | list   | `[]`                                       |             |
| metrics.enable                                   | bool   | `true`                                     |             |
| metrics.port                                     | int    | `8443`                                     |             |
| prometheus.enable                                | bool   | `false`                                    |             |
| rbacHelpers.enable                               | bool   | `false`                                    |             |
| webhook.enable                                   | bool   | `true`                                     |             |
| webhook.port                                     | int    | `9443`                                     |             |
