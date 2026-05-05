# Lookout Helm chart — deployment examples

The chart supports three Gateway models, controlled by `gateway.provider`. Pick
the one that matches your cluster's networking layer.

## 1. Envoy Gateway (default — backwards-compatible)

The chart creates the `Gateway`, `BackendTrafficPolicy`,
`ClientTrafficPolicy`, and (optionally) `SecurityPolicy` for basic auth.

```yaml
gateway:
  enabled: true
  provider: envoy-gateway   # default; can be omitted
global:
  gatewayAPI:
    gatewayClassName: eg

httproute:
  hostnames:
    - lookout.example.com

basicAuth:
  enabled: true             # renders SecurityPolicy + ExternalSecret
  externalSecret:
    enabled: true
    secretStoreName: my-secret-store
    remoteKey: lookout/basic-auth
```

Behaviour: same as the chart has always rendered — no migration needed.

## 2. Istio (or any external Gateway-API provider)

The chart skips Envoy-Gateway-specific CRDs and HTTPRoute attaches to a
`Gateway` provided by the platform (e.g., `platform-gateway` in
`istio-system`). Auth is expected to be handled upstream — typically an
oauth2-proxy in front of the HTTPRoute, the same pattern as Jaeger/Grafana.

```yaml
gateway:
  enabled: false            # don't create a Gateway; use the platform one
  provider: external

httproute:
  parentRefs:
    - name: platform-gateway
      namespace: istio-system
      sectionName: https
  hostnames:
    - lookout.example.com

basicAuth:
  enabled: false            # auth handled by oauth2-proxy upstream
```

Notes for Istio meshes:
- Dgraph uses gRPC + custom TCP ports; mTLS sidecar interception breaks it.
  Annotate Dgraph pods to skip injection (set
  `dgraph.zero.podAnnotations` and `dgraph.alpha.podAnnotations` with
  `sidecar.istio.io/inject: "false"`), or label the namespace
  `istio-injection: disabled`.
- For rate limiting / body-size limits previously provided by
  `BackendTrafficPolicy` / `ClientTrafficPolicy`, use Istio
  `EnvoyFilter` or the analogous mesh-level policies.

## 3. Bring your own gateway in another namespace

For clusters with a centrally-managed gateway you don't want to touch from
this chart at all:

```yaml
gateway:
  enabled: false
  provider: external

httproute:
  parentRefs:
    - name: my-gateway
      namespace: gateway-system
  hostnames:
    - lookout.internal

basicAuth:
  enabled: false
```

## Backward-compatibility guarantee

`gateway.provider` defaults to `envoy-gateway` and `httproute.parentRefs`
defaults to `[]`. Existing values files render byte-identical Kubernetes
manifests; this refactor is opt-in.
