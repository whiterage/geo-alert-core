# Geo Alert Core

A geo-alerting backend in Go: check a user's coordinates against active danger zones, get back
whether they're inside one, and have the service fire a webhook to a downstream system (built
against a Django news portal) when they are.

## Architecture

Clean Architecture, four layers:

- **Handler** — HTTP layer (Gin)
- **Service** — business logic
- **Repository** — data access (PostgreSQL/PostGIS)
- **Infrastructure** — external dependencies (Redis, webhooks)

## Stack

- Go 1.24+
- PostgreSQL 15 with the PostGIS extension
- Redis, for caching active incidents
- Docker & Docker Compose

## Quick start

```bash
git clone https://github.com/whiterage/geo-alert-core.git
cd geo-alert-core
cp .env.example .env
docker-compose up -d
make migrate-up
```

The service starts on `http://localhost:8080`.

Without Docker:

```bash
make deps
make run
# or: go run ./cmd/server
```

```bash
make help          # list all targets
make test          # go test ./...
make test-coverage
make migrate-up / migrate-down
make docker-up / docker-down / docker-logs
```

## API

**Public**

| Method | Path | Does |
| --- | --- | --- |
| `GET` | `/api/v1/system/health` | Health check |
| `POST` | `/api/v1/location/check` | Check a lat/lon against active incidents |

**Protected** (`Authorization: Bearer <api-key>` or `X-API-Key: <api-key>`)

| Method | Path | Does |
| --- | --- | --- |
| `POST` | `/api/v1/incidents` | Create an incident (danger zone) |
| `GET` | `/api/v1/incidents` | List incidents, paginated |
| `GET` | `/api/v1/incidents/{id}` | Get one incident |
| `PUT` | `/api/v1/incidents/{id}` | Update an incident |
| `DELETE` | `/api/v1/incidents/{id}` | Deactivate an incident |
| `GET` | `/api/v1/incidents/stats?minutes=60` | Per-zone check counts over a time window |

```bash
curl -X POST http://localhost:8080/api/v1/location/check \
  -H "Content-Type: application/json" \
  -d '{"user_id":"user123","latitude":55.7558,"longitude":37.6173}'
```

```json
{
  "has_danger": true,
  "incidents": [
    { "id": "uuid", "title": "Danger zone", "latitude": 55.7558, "longitude": 37.6173, "radius": 100.0, "is_active": true }
  ]
}
```

```bash
curl -X POST http://localhost:8080/api/v1/incidents \
  -H "Authorization: Bearer your-api-key" -H "Content-Type: application/json" \
  -d '{"title":"Danger zone","latitude":55.7558,"longitude":37.6173,"radius":100.0}'
```

## How the interesting parts work

**Geospatial lookup.** Incident coordinates are indexed with a PostGIS GIST index, and the
location check queries with `ST_DWithin(geography, geography, radius)` — a database-side radius
search rather than pulling every incident into Go and filtering there.

**Caching.** Active incidents are cached in Redis for 5 minutes to keep the hot path (checking a
coordinate) off the database on repeated checks.

**Webhooks are async and retried.** Firing a webhook happens in its own goroutine, so it never
blocks the response to the location check. Failed deliveries retry with backoff:
attempt 1 immediately, then after 5s, 10s, 20s.

**SQL injection.** All queries are parameterized (`$1`, `$2`, …) — no string-built SQL anywhere in
the repository layer.

## Testing webhooks locally with ngrok

```bash
brew install ngrok        # or https://ngrok.com/download
ngrok http 9090
```

Point the service at the tunnel:

```env
WEBHOOK_URL=https://abc123.ngrok.io/webhook
```

Then run something to receive it — a one-file example:

```python
from http.server import HTTPServer, BaseHTTPRequestHandler
import json

class WebhookHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        body = self.rfile.read(int(self.headers['Content-Length']))
        print(json.dumps(json.loads(body), indent=2))
        self.send_response(200)
        self.end_headers()

HTTPServer(('localhost', 9090), WebhookHandler).serve_forever()
```

## Project layout

```
cmd/server/            entry point
internal/
  config/               environment-based configuration
  domain/                domain models
  handler/               HTTP handlers (Gin)
  service/               business logic, caching
  repository/            PostgreSQL/PostGIS queries
  infrastructure/
    postgres/             DB connection
    redis/                cache client
    webhook/              async sender with retry/backoff
  middleware/             API key auth
migrations/              SQL schema
```

## Troubleshooting

| Symptom | Check |
| --- | --- |
| "Failed to connect to database" | PostgreSQL running? PostGIS extension installed? `.env` values correct? |
| "Failed to connect to Redis" | Redis running? `.env` values correct? |
| "API_KEY is required" | Set `API_KEY` in `.env` |
| Webhooks never arrive | `WEBHOOK_URL` correct? ngrok tunnel still up? check app logs |
