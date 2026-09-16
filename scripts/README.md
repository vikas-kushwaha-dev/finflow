# FinFlow Scripts

## Local CI checks

Run from the project root:

```powershell
.\scripts\check.ps1
```

The script runs all Go tests, builds every service command, validates the Docker Compose configuration, and tests both development and release Kubernetes manifest rendering. These are the same checks enforced by GitHub Actions.

## Docker smoke test

Run from the project root:

```powershell
.\scripts\smoke.ps1
```

The script starts Docker Compose by default, creates a payment through the gateway, waits for ledger entries, checks the entries are balanced, and confirms direct service API calls are blocked without the internal service token.

To run against an already-started stack:

```powershell
.\scripts\smoke.ps1 -SkipComposeUp
```

To verify a deployed environment through its public gateway when service ports are intentionally private:

```powershell
.\scripts\smoke.ps1 `
  -SkipComposeUp `
  -GatewayOnly `
  -GatewayUrl "https://finflow.example.com" `
  -ApiKey "YOUR_RELEASE_API_KEY"
```

Gateway-only mode still creates and replays a payment and waits for balanced ledger entries. It skips direct service readiness and internal-port authentication checks.

If Docker image downloads are slow or fail, run the script again after Docker Desktop finishes pulling `postgres`, `apache/kafka`, `golang`, and `alpine` images.

The smoke test is a release check rather than a pull-request CI check because it starts PostgreSQL, Kafka, and every FinFlow service. Run it before a release or after changes to cross-service behavior, event delivery, Dockerfiles, or Docker Compose wiring.

## Release manifest rendering

`render-release-manifests.ps1` renders the Kubernetes application and migration resources with immutable container image digests. The release workflow calls it after publishing all three images. It can also be run locally with a registry path and three valid `sha256` digests.
