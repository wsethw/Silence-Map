# Silence Map

Silence Map is a collaborative geospatial application for finding quiet public places in real time. People can submit quietness reports, confirm nearby reports, search the current visible map area, and watch live updates arrive through WebSocket.

The project is intentionally a polished portfolio monolith: one Go service, PostgreSQL/PostGIS, an in-memory realtime hub, and a dependency-light Leaflet frontend.

## Stack

- Go 1.22+
- PostgreSQL 16 + PostGIS 3.4
- `database/sql` with the pgx driver
- chi router
- gorilla/websocket
- Leaflet 1.9.4 with CARTO Dark Matter tiles
- Vanilla HTML/CSS/JavaScript

## Architecture

```text
cmd/server             HTTP bootstrap, middleware, DB pool, static files
internal/domain        Report, Confirmation, Point, Bounds, errors
internal/usecase       Application rules and query contracts
internal/handler       REST parsing, validation, rate limiting
internal/repository    Postgres/PostGIS SQL implementation
internal/websocket     Single-instance in-memory realtime hub
internal/identity      Signed anonymous demo session middleware
internal/ratelimit     In-memory fixed-window limiter
web/                   Leaflet frontend, CSS, JavaScript, static checks
db/init.sql            PostGIS schema and temporal decay function
```

## What It Demonstrates

- Spatial search with `GEOGRAPHY(POINT, 4326)`, `ST_DWithin`, and viewport envelopes.
- Bounds-only querying for large map viewports so "Search this area" never silently truncates the visible map.
- Time-decayed quietness ranking with confirmation-weighted reports.
- Realtime collaborative updates filtered by subscribed map bounds.
- Signed anonymous demo identity instead of trusting client-provided `user_id`.
- Write rate limiting using anonymous identity plus a hashed normalized client IP signal.
- Accessible modal behavior, keyboard report creation, safe DOM rendering, and responsive mobile layout.

## Environment Setup

`.env.example` is a reference file. The Go app reads process environment variables directly; it does not load `.env` by itself.

PowerShell:

```powershell
$env:HTTP_ADDR=":8080"
$env:APP_TIMEZONE="America/Sao_Paulo"
$env:DB_HOST="localhost"
$env:DB_PORT="5432"
$env:DB_USER="postgres"
$env:DB_PASSWORD="postgres"
$env:DB_NAME="silence_map"
$env:DB_SSLMODE="disable"
$env:SESSION_SECRET="replace-with-a-long-random-secret"
go run ./cmd/server
```

Linux/macOS:

```bash
export HTTP_ADDR=:8080
export APP_TIMEZONE=America/Sao_Paulo
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=postgres
export DB_NAME=silence_map
export DB_SSLMODE=disable
export SESSION_SECRET=replace-with-a-long-random-secret
go run ./cmd/server
```

Docker Compose sets the required service environment inside `docker-compose.yml`.

## Run With Docker Compose

```powershell
docker compose up --build
```

Open [http://localhost:8080](http://localhost:8080).

Health endpoints:

- `GET /healthz` checks that the HTTP server is alive.
- `GET /readyz` also checks database readiness.

## Local Frontend Demo

The recommended workflow is serving the app from Go at [http://localhost:8080](http://localhost:8080).

Direct `file://` mode is also supported for demo inspection when the backend is running on `localhost:8080`. The server allows only restricted demo CORS origins:

- `Origin: null` for local `file://`
- `http://localhost:8080`
- `http://127.0.0.1:8080`

Credentials are allowed so the signed anonymous session cookie still works. Broad CORS is not enabled.

## API Examples

Create a report. Any `user_id` in the body is ignored; the server uses the signed anonymous session identity.

```powershell
curl.exe -i -X POST http://localhost:8080/api/reports `
  -H "Content-Type: application/json" `
  -d "{\"latitude\":-23.5505,\"longitude\":-46.6333,\"quietness\":5,\"place_name\":\"Central Library\"}"
```

Confirm a report:

```powershell
curl.exe -i -X POST http://localhost:8080/api/reports/<REPORT_ID>/confirm
```

Recent reports by radius:

```powershell
curl.exe "http://localhost:8080/api/reports/recent?lat=-23.5505&lng=-46.6333&radius=5000"
```

Recent reports by viewport only:

```powershell
curl.exe "http://localhost:8080/api/reports/recent?lat=-23.5505&lng=-46.6333&radius=0&north=-22.8&south=-24.3&east=-45.9&west=-47.4"
```

Quiet-place search for a normal viewport:

```powershell
curl.exe "http://localhost:8080/api/places/quiet?lat=-23.5505&lng=-46.6333&radius=5000&north=-23.4&south=-23.7&east=-46.5&west=-46.8&day_of_week=6&hour=15"
```

Quiet-place search for a large viewport:

```powershell
curl.exe "http://localhost:8080/api/places/quiet?lat=-23.5505&lng=-46.6333&radius=0&north=-22.8&south=-24.3&east=-45.9&west=-47.4&day_of_week=6&hour=15"
```

Bounds are optional, but when used all four values (`north`, `south`, `east`, `west`) must be sent together.

## How "Search This Area" Works

The frontend computes the distance from the map center to each viewport corner.

- If the viewport fits within the 50 km radius limit, it sends both radius and bounds.
- If the viewport is larger than 50 km, it sends `radius=0` plus bounds.
- The backend accepts `radius=0` only when valid bounds are present.
- SQL skips `ST_DWithin` in bounds-only mode and filters with `ST_MakeEnvelope` instead.

This keeps center-radius searches protected by a safe maximum radius while preserving the meaning of "Search this area" on large screens or low zoom levels.

## Quiet-Place Ranking

Reports newer than 24 hours are ranked with:

- quietness level
- confirmation count as collaboration weight
- temporal decay after 2 hours
- day-of-week and hour affinity
- deterministic tie-breakers in SQL

The query groups nearby reports into roughly city-block cells with `ST_SnapToGrid(..., 0.001)`. This is a deliberate demo tradeoff: simple, explainable clustering without adding H3 or another dependency.

## WebSocket Behavior

Connect to:

```text
ws://localhost:8080/ws
```

When the app is served over HTTPS, it uses `wss://current-host/ws`. When served over HTTP, it uses `ws://current-host/ws`. Direct `file://` mode falls back to `ws://localhost:8080/ws`.

Subscribe to the current viewport:

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
{ "type": "error", "code": "invalid_subscription", "message": "expected subscribe action with valid bounds" }
```

The hub is in-memory and single-instance only. A horizontally scaled production version would use Redis Pub/Sub, NATS, PostgreSQL `LISTEN/NOTIFY`, or a dedicated event bus.

## Security Notes

- Client-provided `user_id` is ignored.
- Signed anonymous session cookies identify demo users.
- Rate limiting combines anonymous identity with a hashed normalized IP signal.
- Self-confirmation and duplicate confirmation are rejected.
- JSON decoding is strict and request bodies are size-limited.
- Errors are returned as structured JSON without stack traces.
- Frontend rendering uses DOM APIs and `textContent`; user-controlled place names are not injected with HTML.
- CSP removes inline script allowances. `style-src 'unsafe-inline'` remains because Leaflet and map markers rely on runtime inline positioning/styles.
- `.env` and local secret files are ignored by git. `.env.example` is intentionally committed.

This is demo identity, not production authentication. A production product should add real auth, abuse monitoring, stronger IP trust boundaries, and persistent distributed rate limiting.

## Testing

```powershell
go test ./...
node web/index.test.js
```

Or:

```powershell
make test
```

The Go tests cover validation, partial/invalid bounds, bounds-only search, large viewport semantics, trusted identity, rate-limit keying, CORS, duplicate/self-confirmation propagation, and handler error behavior. The frontend static checks verify URL generation, viewport query semantics, local fallback filtering/ranking, confirmation rollback code paths, modal accessibility hooks, XSS-safe rendering patterns, responsive CSS, and English UI text.

## Manual Verification Checklist

Desktop:

- Open `http://localhost:8080`.
- Confirm the dark CARTO map loads with demo markers.
- Pan and zoom; existing reports should reload for the visible area.
- Click "Search this area"; results should match the current viewport.
- Click the map and submit a report.
- Open a second tab and verify realtime updates.

Tablet:

- Verify the left glass panel remains readable.
- Search, report, and confirm reports by touch.
- Rotate between portrait and landscape.

Mobile portrait:

- Verify the panel becomes a bottom sheet.
- Toggle the sheet open and closed.
- Use "Report at map center" with the keyboard or touch.
- Open the modal, tab through fields, close with Escape if a hardware keyboard is available.

Mobile landscape:

- Verify the panel does not cover the entire map.
- Verify the modal fits when the keyboard opens.
- Confirm no horizontal scrolling appears.

Direct file demo:

- Start the Go backend on `localhost:8080`.
- Open `web/index.html` directly.
- Verify REST requests and WebSocket use localhost fallback and cookies are accepted through restricted CORS.

## Troubleshooting

- Map is blank: check requests to `basemaps.cartocdn.com` in the browser network tab.
- Realtime says reconnecting: verify the Go server is reachable and `/ws` is not blocked.
- Direct file mode cannot write: confirm the backend is running on `localhost:8080`.
- Database readiness fails: run `docker compose logs -f db api` and check `/readyz`.
- Duplicate confirmation returns conflict: this is expected for the same anonymous session.

## Demo Limitations

- Anonymous identity can still be reset by clearing cookies.
- Rate limiting is in-memory and resets on process restart.
- WebSocket fanout is single-instance.
- Demo markers are included so screenshots are meaningful before community data exists.
