# FinFlow Scripts

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
