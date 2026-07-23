{{/*
Expand the name of the chart.
*/}}
{{- define "renovate-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "renovate-operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains .Release.Name $name }}
{{- $name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Resource name with proper truncation for Kubernetes 63-character limit.
Takes a dict with:
  - .suffix: Resource name suffix (e.g., "metrics", "webhook")
  - .context: Template context (root context with .Values, .Release, etc.)
Dynamically calculates safe truncation to ensure total name length <= 63 chars.
*/}}
{{- define "renovate-operator.resourceName" -}}
{{- $fullname := include "renovate-operator.fullname" .context }}
{{- $suffix := .suffix }}
{{- $maxLen := sub 62 (len $suffix) | int }}
{{- if gt (len $fullname) $maxLen }}
{{- printf "%s-%s" (trunc $maxLen $fullname | trimSuffix "-") $suffix | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" $fullname $suffix | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{/*
ServiceAccount name to use.
If serviceAccount.enabled is false and serviceAccount.name is set, use that name.
Otherwise, use the standard resourceName helper with "controller-manager" suffix.
*/}}
{{- define "renovate-operator.serviceAccountName" -}}
{{- if and (not (.Values.serviceAccount.enabled | default true)) .Values.serviceAccount.name }}
{{- .Values.serviceAccount.name }}
{{- else }}
{{- include "renovate-operator.resourceName" (dict "suffix" "controller-manager" "context" .) }}
{{- end }}
{{- end }}

{{/*
Standard Kubernetes metadata labels.
Takes the root context ($) as the single argument.
Add component and extra labels inline in each template.
*/}}
{{- define "renovate-operator.labels" -}}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/name: {{ include "renovate-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Pod selector labels for component resources.
Takes the root context ($) as the single argument.
*/}}
{{- define "renovate-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "renovate-operator.name" . }}
control-plane: controller-manager
{{- end }}

{{/*
Component Service spec.
Takes a dict with:
  - .svc: service values dict
  - .component: component name (for default port name and targetPort)
  - .context: root context ($)
*/}}
{{- define "renovate-operator.serviceSpec" -}}
{{- $svc := .svc }}
{{- $component := .component }}
{{- $context := .context }}
type: {{ $svc.type }}
{{- with $svc.clusterIP }}
clusterIP: {{ . | quote }}
{{- end }}
{{- if $svc.sessionAffinity }}
sessionAffinity: {{ $svc.sessionAffinity }}
{{- end }}
selector:
  {{- include "renovate-operator.selectorLabels" $context | nindent 2 }}
ports:
  - name: {{ $svc.targetPort | default $component }}
    port: {{ $svc.port }}
    targetPort: {{ $svc.targetPort | default $component }}
    protocol: TCP
    {{- if and (eq $svc.type "NodePort") $svc.nodePort }}
    nodePort: {{ $svc.nodePort }}
    {{- end }}
{{- end }}

{{/*
Component Ingress spec.
Takes a dict with:
  - .ingress: ingress values dict
  - .svcPort: int (service port number)
  - .fullname: resource fullname (for backend service name)
*/}}
{{- define "renovate-operator.ingressSpec" -}}
{{- $ingress := .ingress }}
{{- $svcPort := .svcPort }}
{{- $fullname := .fullname }}
{{- with $ingress.className }}
ingressClassName: {{ . | quote }}
{{- end }}
{{- if $ingress.tls }}
tls:
  {{- range $ingress.tls }}
  - hosts:
      {{- range .hosts }}
      - {{ . | quote }}
      {{- end }}
    secretName: {{ .secretName | quote }}
  {{- end }}
{{- end }}
rules:
  {{- if not $ingress.hosts }}
  - http:
      paths:
        - path: /
          pathType: Prefix
          backend:
            service:
              name: {{ $fullname }}
              port:
                number: {{ $svcPort }}
  {{- end }}
  {{- range $h := $ingress.hosts }}
  - host: {{ $h.host | quote }}
    http:
      paths:
        {{- range $p := $h.paths }}
        - path: {{ $p.path | quote }}
          pathType: {{ default "Prefix" $p.pathType | quote }}
          backend:
            service:
              name: {{ $fullname }}
              port:
                number: {{ $svcPort }}
        {{- end }}
  {{- end }}
{{- end }}

{{/*
Component Gateway spec.
Takes a dict with:
  - .gw: gateway values dict
  - .component: component name (for error messages)
*/}}
{{- define "renovate-operator.gatewaySpec" -}}
{{- $gw := .gw }}
{{- $component := .component }}
gatewayClassName: {{ required (printf "%s.gateway.className is required when %s.gateway.gatewayName is not set" $component $component) $gw.className | quote }}
{{- with $gw.addresses }}
addresses:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- if $gw.listeners }}
listeners:
  {{- range $i, $l := $gw.listeners }}
  - name: {{ $l.name | default (printf "http-%d" $i) | quote }}
    port: {{ required "listeners[].port is required" $l.port }}
    protocol: {{ $l.protocol | default "HTTP" | quote }}
    {{- if $l.hostname }}
    hostname: {{ $l.hostname | quote }}
    {{- end }}
    {{- with $l.tls }}
    tls:
      {{- toYaml . | nindent 6 }}
    {{- end }}
    {{- with $l.allowedRoutes }}
    allowedRoutes:
      {{- toYaml . | nindent 6 }}
    {{- end }}
  {{- end }}
{{- else }}
listeners:
  - name: http
    port: 80
    protocol: HTTP
{{- end }}
{{- end }}

{{/*
Component HTTPRoute spec.
Takes a dict with:
  - .gw: gateway values dict
  - .svcPort: int (service port number)
  - .fullname: resource fullname (for backendRefs)
  - .gwName: gateway name (for parentRefs)
  - .gwNamespace: gateway namespace (for parentRefs)
*/}}
{{- define "renovate-operator.httpRouteSpec" -}}
{{- $gw := .gw }}
{{- $svcPort := .svcPort }}
{{- $fullname := .fullname }}
{{- $gwName := .gwName }}
{{- $gwNamespace := .gwNamespace }}
{{- $ownGW := not $gw.gatewayName }}
{{- $sectionName := $gw.sectionName }}
{{- if and $ownGW (not $sectionName) }}
  {{- if $gw.listeners }}
    {{- $sectionName = (index $gw.listeners 0).name }}
  {{- else }}
    {{- $sectionName = "http" }}
  {{- end }}
{{- end }}
parentRefs:
  - name: {{ $gwName }}
    namespace: {{ $gwNamespace }}
    {{- if $sectionName }}
    sectionName: {{ $sectionName | quote }}
    {{- end }}
{{- with $gw.hosts }}
hostnames:
  {{- range . }}
  - {{ . | quote }}
  {{- end }}
{{- end }}
rules:
  - matches:
      {{- if not $gw.paths }}
      - path:
          type: PathPrefix
          value: /
      {{- end }}
      {{- range $p := $gw.paths }}
      - path:
          type: {{ default "PathPrefix" $p.pathType | quote }}
          value: {{ $p.path | quote }}
      {{- end }}
    {{- with $gw.filters }}
    filters:
      {{- toYaml . | nindent 6 }}
    {{- end }}
    backendRefs:
      - name: {{ $fullname }}
        port: {{ $svcPort }}
    {{- with $gw.timeouts }}
    timeouts:
      {{- toYaml . | nindent 6 }}
    {{- end }}
{{- end }}
