# Kubernetes deployment foundation

These manifests deploy the FinFlow application processes only. PostgreSQL and Kafka are external dependencies and are not deployed as single-node in-cluster workloads.

## Prerequisites

- A Kubernetes cluster, `kubectl`, and standalone `kustomize` for changing release image references
- Reachable PostgreSQL and Kafka endpoints
- Gateway, payment service, and ledger service images in a registry accessible to the cluster

## Configure

Update `configmap.yaml` with the external Kafka broker addresses. The internal payment and ledger URLs already use Kubernetes Service discovery.

For manual deployments, set explicit image references before deploying. Tagged releases generate digest-pinned manifests automatically and are preferred. To edit the development manifests manually:

```powershell
kubectl kustomize infrastructure/kubernetes
cd infrastructure/kubernetes
kustomize edit set image finflow/gateway-service=registry.example.com/finflow/gateway-service:1.0.0
kustomize edit set image finflow/payment-service=registry.example.com/finflow/payment-service:1.0.0
kustomize edit set image finflow/ledger-service=registry.example.com/finflow/ledger-service:1.0.0
cd migration
kustomize edit set image finflow/payment-service=registry.example.com/finflow/payment-service:1.0.0
```

Do not commit credentials. Create the required Secret directly in the cluster or through the production secret manager:

If the published GHCR packages are private, configure an `imagePullSecret` on the `finflow` namespace's default ServiceAccount before rollout.

```powershell
kubectl create namespace finflow --dry-run=client -o yaml | kubectl apply -f -
kubectl -n finflow create secret generic finflow-secrets `
  --from-literal=database-url="postgres://USER:PASSWORD@HOST:5432/finflow?sslmode=require" `
  --from-literal=api-key="REPLACE_ME" `
  --from-literal=internal-service-token="REPLACE_ME"
```

## Deploy

Apply the namespace, configuration, APIs, services, and workers:

```powershell
kubectl apply -k infrastructure/kubernetes
```

Run migrations once for each release before routing traffic to the new application version:

```powershell
kubectl -n finflow delete job finflow-migrate --ignore-not-found
kubectl create -k infrastructure/kubernetes/migration
kubectl -n finflow wait --for=condition=complete job/finflow-migrate --timeout=5m
kubectl -n finflow get jobs
```

The migration Job has a separate Kustomize target and is intentionally not part of the application deployment. Its completed object is retained for 24 hours for inspection; remove a previous completed Job before creating the next release run.

## Exposure and operations

The gateway Service uses `LoadBalancer`. Payment and ledger Services use `ClusterIP` and should remain internal. API Deployments have two replicas, health probes, resource boundaries, non-root execution, read-only root filesystems, and dropped Linux capabilities.

The worker Deployments start with one replica. Increase consumer replicas only after considering Kafka partition count. Keep the outbox publisher at one replica until its database claim behavior has been load-tested for concurrent publishers.

The current Kafka client configuration supports broker addresses but does not yet configure SASL or TLS. Do not connect these workloads to a production Kafka cluster until transport authentication is implemented.

## Tagged releases

Pushing a semantic version tag such as `v1.0.0` starts `.github/workflows/release.yml`. The workflow:

1. Runs the complete CI workflow.
2. Builds each image and blocks publishing when Trivy finds a fixed HIGH or CRITICAL vulnerability.
3. Publishes `linux/amd64` and `linux/arm64` images to GitHub Container Registry with version and commit tags.
4. Records SBOM and provenance data.
5. Creates a GitHub release containing application and migration manifests pinned to immutable image digests.

Use the generated release assets in this order:

```powershell
kubectl apply -f infrastructure/kubernetes/namespace.yaml
kubectl -n finflow apply -f infrastructure/kubernetes/configmap.yaml

# Create or update finflow-secrets through the cluster's secret manager.

kubectl -n finflow delete job finflow-migrate --ignore-not-found
kubectl create -f finflow-migration.yaml
kubectl -n finflow wait --for=condition=complete job/finflow-migrate --timeout=5m
kubectl apply -f finflow-kubernetes.yaml
kubectl -n finflow rollout status deployment/gateway --timeout=5m
kubectl -n finflow rollout status deployment/payment-service --timeout=5m
kubectl -n finflow rollout status deployment/ledger-service --timeout=5m
```

After the rollout, run the smoke test against the public gateway with `-SkipComposeUp -GatewayOnly`. Rollback should use the previous release's digest-pinned `finflow-kubernetes.yaml`; database migrations must be designed to remain backward compatible because automatic schema rollback is not provided.
