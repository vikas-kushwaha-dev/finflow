# FinFlow Scripts

## Local CI checks

Run from the project root:

```powershell
.\scripts\check.ps1
```

The script runs all Go tests, builds every service command, and validates the Docker Compose configuration. These are the same checks enforced by GitHub Actions.

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

If Docker image downloads are slow or fail, run the script again after Docker Desktop finishes pulling `postgres`, `apache/kafka`, `golang`, and `alpine` images.

The smoke test is a release check rather than a pull-request CI check because it starts PostgreSQL, Kafka, and every FinFlow service. Run it before a release or after changes to cross-service behavior, event delivery, Dockerfiles, or Docker Compose wiring.
