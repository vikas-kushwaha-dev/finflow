# Docker

The active Docker Compose file is currently at the project root for a simple local developer workflow:

```bash
docker compose up -d
```

This starts PostgreSQL, runs migrations once, and then starts the payment service.

Useful commands:

```bash
docker compose ps
docker compose logs -f payment-service
docker compose down
```
