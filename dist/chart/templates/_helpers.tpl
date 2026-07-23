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
{{- if and (not .Values.serviceAccount.enabled) .Values.serviceAccount.name }}
{{- .Values.serviceAccount.name }}
{{- else }}
{{- include "renovate-operator.resourceName" (dict "suffix" "controller-manager" "context" .) }}
{{- end }}
{{- end }}

{{/*
Common metadata labels for component resources.
Takes a dict with:
  - .context: root context ($)
  - .component: label value (e.g. "frontend", "receiver")
  - .extraLabels: optional map of additional labels
*/}}
{{- define "renovate-operator.componentLabels" -}}
app.kubernetes.io/managed-by: {{ .context.Release.Service }}
app.kubernetes.io/name: {{ include "renovate-operator.name" .context }}
helm.sh/chart: {{ .context.Chart.Name }}-{{ .context.Chart.Version | replace "+" "_" }}
app.kubernetes.io/instance: {{ .context.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- if .extraLabels }}
{{ toYaml .extraLabels -}}
{{- end }}
{{- end }}

{{/*
Component Service resource.
Takes a dict with:
  - .context: root context ($)
  - .enabled: service.enabled
  - .managerEnabled: .Values.manager.enabled
  - .component: component name
  - .componentEnabled: .Values.<component>.enabled
  - .svc: service values dict
*/}}
{{- define "renovate-operator.componentService" -}}
{{- if and .managerEnabled .componentEnabled .enabled }}
{{- $component := .component }}
{{- $context := .context }}
{{- $svc := .svc }}
{{- $fullname := include "renovate-operator.resourceName" (dict "suffix" $component "context" $context) }}
apiVersion: v1
kind: Service
metadata:
  name: {{ $fullname }}
  namespace: {{ $context.Release.Namespace }}
  labels:
    {{- include "renovate-operator.componentLabels" (dict "component" $component "context" $context "extraLabels" $svc.labels) | nindent 4 }}
  {{- with $svc.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  type: {{ $svc.type }}
  {{- with $svc.clusterIP }}
  clusterIP: {{ . | quote }}
  {{- end }}
  {{- if $svc.sessionAffinity }}
  sessionAffinity: {{ $svc.sessionAffinity }}
  {{- end }}
  selector:
    app.kubernetes.io/name: {{ include "renovate-operator.name" $context }}
    control-plane: controller-manager
  ports:
    - name: {{ $svc.targetPort | default $component }}
      port: {{ $svc.port }}
      targetPort: {{ $svc.targetPort | default $component }}
      protocol: TCP
      {{- if and (eq $svc.type "NodePort") $svc.nodePort }}
      nodePort: {{ $svc.nodePort }}
      {{- end }}
{{- end }}
{{- end }}

{{/*
Component Ingress resource.
Takes a dict with:
  - .context: root context ($)
  - .enabled: ingress.enabled
  - .managerEnabled: .Values.manager.enabled
  - .component: component name
  - .componentEnabled: .Values.<component>.enabled
  - .ingress: ingress values dict
  - .svcPort: int (service port number)
*/}}
{{- define "renovate-operator.componentIngress" -}}
{{- if and .managerEnabled .componentEnabled .enabled }}
{{- $component := .component }}
{{- $context := .context }}
{{- $ingress := .ingress }}
{{- $svcPort := .svcPort }}
{{- $fullname := include "renovate-operator.resourceName" (dict "suffix" $component "context" $context) }}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ $fullname }}
  namespace: {{ $context.Release.Namespace }}
  labels:
    {{- include "renovate-operator.componentLabels" (dict "component" $component "context" $context) | nindent 4 }}
  {{- with $ingress.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
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
{{- end }}

{{/*
Component Gateway resource.
Takes a dict with:
  - .context: root context ($)
  - .enabled: gateway.enabled
  - .managerEnabled: .Values.manager.enabled
  - .component: component name
  - .componentEnabled: .Values.<component>.enabled
  - .gw: gateway values dict
*/}}
{{- define "renovate-operator.componentGateway" -}}
{{- if and .managerEnabled .componentEnabled .enabled }}
{{- $component := .component }}
{{- $context := .context }}
{{- $gw := .gw }}
{{- $fullname := include "renovate-operator.resourceName" (dict "suffix" $component "context" $context) }}
{{- $gwName := default (printf "%s-gateway" $fullname) $gw.gatewayName }}
{{- $gwNamespace := default $context.Release.Namespace $gw.gatewayNamespace }}
{{- if not $gw.gatewayName }}
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: {{ $gwName }}
  namespace: {{ $gwNamespace }}
  labels:
    {{- include "renovate-operator.componentLabels" (dict "component" $component "context" $context "extraLabels" $gw.labels) | nindent 4 }}
  {{- with $gw.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  gatewayClassName: {{ required (printf "%s.gateway.className is required when %s.gateway.gatewayName is not set" $component $component) $gw.className | quote }}
  {{- with $gw.addresses }}
  addresses:
    {{- toYaml . | nindent 4 }}
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
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with $l.allowedRoutes }}
      allowedRoutes:
        {{- toYaml . | nindent 8 }}
      {{- end }}
    {{- end }}
  {{- else }}
  listeners:
    - name: http
      port: 80
      protocol: HTTP
  {{- end }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Component HTTPRoute resource.
Takes a dict with:
  - .context: root context ($)
  - .enabled: gateway.enabled
  - .managerEnabled: .Values.manager.enabled
  - .component: component name
  - .componentEnabled: .Values.<component>.enabled
  - .gw: gateway values dict
  - .svcPort: int (service port number)
*/}}
{{- define "renovate-operator.componentHTTPRoute" -}}
{{- if and .managerEnabled .componentEnabled .enabled }}
{{- $component := .component }}
{{- $context := .context }}
{{- $gw := .gw }}
{{- $svcPort := .svcPort }}
{{- $fullname := include "renovate-operator.resourceName" (dict "suffix" $component "context" $context) }}
{{- $gwName := default (printf "%s-gateway" $fullname) $gw.gatewayName }}
{{- $gwNamespace := default $context.Release.Namespace $gw.gatewayNamespace }}
{{- $ownGW := not $gw.gatewayName }}
{{- $sectionName := $gw.sectionName }}
{{- if and $ownGW (not $sectionName) }}
  {{- if $gw.listeners }}
    {{- $sectionName = (index $gw.listeners 0).name }}
  {{- else }}
    {{- $sectionName = "http" }}
  {{- end }}
{{- end }}
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: {{ $fullname }}
  namespace: {{ $context.Release.Namespace }}
  labels:
    {{- include "renovate-operator.componentLabels" (dict "component" $component "context" $context "extraLabels" $gw.labels) | nindent 4 }}
  {{- with $gw.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
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
        {{- toYaml . | nindent 8 }}
      {{- end }}
      backendRefs:
        - name: {{ $fullname }}
          port: {{ $svcPort }}
      {{- with $gw.timeouts }}
      timeouts:
        {{- toYaml . | nindent 8 }}
      {{- end }}
{{- end }}
{{- end }}

{{/*
Component ReferenceGrant resource.
Takes a dict with:
  - .context: root context ($)
  - .enabled: gateway.enabled
  - .managerEnabled: .Values.manager.enabled
  - .component: component name
  - .componentEnabled: .Values.<component>.enabled
  - .gw: gateway values dict
*/}}
{{- define "renovate-operator.componentReferenceGrant" -}}
{{- if and .managerEnabled .componentEnabled .enabled }}
{{- $component := .component }}
{{- $context := .context }}
{{- $gw := .gw }}
{{- if and $gw.gatewayName $gw.gatewayNamespace }}
{{- $gwNamespace := $gw.gatewayNamespace }}
{{- if ne $gwNamespace $context.Release.Namespace }}
{{- $gwName := $gw.gatewayName }}
{{- $fullname := include "renovate-operator.resourceName" (dict "suffix" $component "context" $context) }}
apiVersion: gateway.networking.k8s.io/v1beta1
kind: ReferenceGrant
metadata:
  name: {{ $fullname }}-gateway
  namespace: {{ $gwNamespace }}
  labels:
    {{- include "renovate-operator.componentLabels" (dict "component" $component "context" $context "extraLabels" $gw.labels) | nindent 4 }}
spec:
  from:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      namespace: {{ $context.Release.Namespace }}
  to:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: {{ $gwName }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
