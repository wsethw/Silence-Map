# Silence Map

Silence Map is a collaborative geospatial demo where people report how quiet a public place feels right now. The backend is a single Go service with clean internal boundaries, PostgreSQL/PostGIS for spatial queries, and WebSocket updates for live collaboration. The frontend is dependency-light HTML/CSS/JavaScript with Leaflet and a dark glassmorphism dashboard UI.

## What It Demonstrates

- PostGIS geography queries with `ST_DWithin` and viewport bounding boxes.
- Realtime updates through an in-memory WebSocket hub.
- Time decay for reports so old noise/quietness data does not dominate current results.
- Anonymous signed-cookie demo identity instead of trusting arbitrary `user_id` values.
- Rate limiting for report creation and confirmations.
- Responsive accessible map UI with keyboard-friendly modal behavior.

## Architecture

```text
cmd/server        HTTP server bootstrap, DB pool, router, graceful shutdown
internal/domain   Report, Confirmation, Point, Bounds, validation primitives
internal/usecase  Application rules and query contracts
internal/handler  REST handlers, request parsing, rate limiting
internal/repository Postgres/PostGIS SQL implementation
internal/websocket In-memory live update hub
internal/identity Signed anonymous demo session middleware
web/              Leaflet frontend and static checks
db/init.sql       PostGIS schema
```

The WebSocket hub is intentionally in-memory for a portfolio monolith. A horizontally scaled production version would move fanout through Redis Pub/Sub, NATS, or PostgreSQL `LISTEN/NOTIFY`.

## Run With Docker Compose

```powershell
cd C:\Users\codwj\OneDrive\Documentos\Project-04\silence-map
docker compose up --build
```

Open [http://localhost:8080](http://localhost:8080).

Health endpoints:

- `GET /healthz` returns when the HTTP server is alive.
- `GET /readyz` also checks database readiness.

## Local Development

Copy `.env.example` to `.env` if you want to run the binary directly. The compose file sets the same values for the container.

```powershell
go mod tidy
go run ./cmd/server
```

The frontend can also be opened as `web/index.html`; in that mode it falls back to `http://localhost:8080` for REST and `ws://localhost:8080/ws` for realtime.

## API Examples

Create a report. The body may include `user_id` for backward compatibility, but the server ignores it and uses a signed anonymous session cookie.

```powershell
curl.exe -i -X POST http://localhost:8080/api/reports `
  -H "Content-Type: application/json" `
  -d "{\"latitude\":-23.5505,\"longitude\":-46.6333,\"quietness\":5,\"place_name\":\"Central Library\"}"
```

Confirm a report:

```powershell
curl.exe -X POST http://localhost:8080/api/reports/<REPORT_ID>/confirm
```

Recent reports in a radius:

```powershell
curl.exe "http://localhost:8080/api/reports/recent?lat=-23.5505&lng=-46.6333&radius=5000"
```

Recent reports constrained to a viewport:

```powershell
curl.exe "http://localhost:8080/api/reports/recent?lat=-23.5505&lng=-46.6333&radius=5000&north=-23.4&south=-23.7&east=-46.5&west=-46.8"
```

Quiet-place search for the visible area:

```powershell
curl.exe "http://localhost:8080/api/places/quiet?lat=-23.5505&lng=-46.6333&radius=5000&north=-23.4&south=-23.7&east=-46.5&west=-46.8&day_of_week=6&hour=15"
```

Bounds are optional, but when used all four values (`north`, `south`, `east`, `west`) must be sent together.

## WebSocket Example

Connect to:

```text
ws://localhost:8080/ws
```

Subscribe to the current map viewport:

```json
{
  "action": "subscribe",
  "bounds": {
    "north": -23.4,
    "south": -23.7,
    "east": -46.5,
    "west": -46.8
  }
}
```

Events:

```json
{ "type": "new_report", "report": { "...": "..." } }
{ "type": "confirmation", "report_id": "...", "quietness": 5, "confirmations": 3 }
```

## Geospatial Notes

PostGIS stores report locations as `GEOGRAPHY(POINT, 4326)`, so radius filtering uses meters. Quiet-place ranking first applies the requested radius, then optionally constrains results to the exact viewport envelope. The query groups reports into roughly 100-meter city-level grid cells using `ST_SnapToGrid(..., 0.001)`, which is a deliberate demo tradeoff: simple, explainable clustering without adding H3 or another dependency.

Reports newer than 2 hours keep full freshness weight. Reports between 2 and 24 hours decay gradually toward half weight. Reports older than 24 hours are ignored by current quiet-place queries.

## Testing

```powershell
go test ./...
node web/index.test.js
```

Or:

```powershell
make test
```

The Go tests cover validation, bounds parsing, trusted demo identity, rate limiting, and usecase behavior. The frontend static test checks JavaScript syntax, dark map tiles, viewport bounds requests, WebSocket URL behavior, focus-trap presence, debounced reload behavior, English UI text, and avoidance of dynamic `innerHTML` assignments.

## Troubleshooting

- If the map does not show, check browser network requests to `basemaps.cartocdn.com`.
- If the status says `Reconnecting...`, the Go server or `/ws` endpoint is not reachable.
- If Docker health checks fail, run `docker compose logs -f api db` and check `/readyz`.
- If a direct `file://` open cannot create reports, ensure the backend is running on `localhost:8080`.

## Production-Like vs Demo-Only

Production-like:

- PostGIS spatial filtering.
- Signed anonymous session cookies.
- REST validation and safe error responses.
- Rate limiting for write endpoints.
- Graceful HTTP shutdown and readiness checks.

Demo-only:

- Anonymous identity is not full authentication.
- WebSocket fanout is single-instance in memory.
- The frontend includes demo markers so portfolio screenshots look useful before any community data exists.
