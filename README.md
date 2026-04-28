# Silence Map

![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)
![PostGIS](https://img.shields.io/badge/PostGIS-Geospatial-2E7D32)
![Leaflet](https://img.shields.io/badge/Leaflet-Interactive%20Maps-199900?logo=leaflet&logoColor=white)
![WebSocket](https://img.shields.io/badge/WebSocket-Realtime-6A5ACD)
![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker&logoColor=white)

A collaborative realtime map that helps people discover **quiet and noisy places nearby**.

Silence Map allows users to report the current noise level of public places, confirm reports from other people, and search for quiet areas directly inside the visible map viewport. It was designed as a portfolio-grade geospatial application combining **Go**, **PostgreSQL/PostGIS**, **Leaflet**, **WebSockets**, responsive UI, accessibility, and safe demo identity handling.

> Instead of mapping traffic, restaurants, or events, this project maps something people actually need every day: **places where they can focus, read, work, study, or simply breathe in silence**.

---

## Preview

```mermaid
flowchart LR
    User["User"] --> UI["Responsive Web App"]
    UI --> Map["Leaflet Map"]
    UI --> API["Go HTTP API"]
    UI <-->|Realtime events| WS["WebSocket Hub"]
    API --> DB["PostgreSQL + PostGIS"]
    API --> Identity["Signed Anonymous Session"]
    API --> RateLimit["Rate Limiting"]
    DB --> Geo["Geospatial Queries"]
    WS --> UI
````

---

## Product Idea

Silence Map is a collaborative platform where people can answer questions like:

* “Where can I study outside home right now?”
* “Is this park quiet enough to read?”
* “Which nearby café is usually calm?”
* “What places are noisy at this time of day?”

Users contribute by reporting whether a location is quiet, moderate, or noisy. Other users can confirm those reports, making the map more reliable over time.

---

## Why This Project Is Different

Most maps show **where things are**.

Silence Map shows **how a place feels right now**.

It focuses on a layer of urban information that is rarely mapped: acoustic comfort.

This makes the project useful for:

* students
* remote workers
* freelancers
* people with noise sensitivity
* readers
* neurodivergent users
* people looking for calm public spaces

---

## Core Features

* Interactive map with realtime silence/noise reports
* Report creation by clicking the map
* Keyboard-accessible “Report at map center” flow
* Quiet-place search based on the current visible area
* Large viewport support with bounds-only geospatial queries
* Realtime updates through WebSockets
* Anonymous signed session identity
* Duplicate and self-confirmation protection
* Rate limiting for public demo safety
* Safe DOM rendering to reduce XSS risk
* Responsive layout for desktop, tablet, and mobile
* Accessible modal with focus trap and focus restoration
* PostGIS-powered geospatial filtering and ranking
* Docker-ready local environment

---

## Tech Stack

| Layer             | Technology                                                  |
| ----------------- | ----------------------------------------------------------- |
| Backend           | Go                                                          |
| Database          | PostgreSQL + PostGIS                                        |
| Frontend          | HTML, CSS, JavaScript                                       |
| Map Engine        | Leaflet                                                     |
| Realtime          | WebSocket                                                   |
| Identity          | Signed anonymous session cookie                             |
| Spatial Queries   | `ST_DWithin`, `ST_MakeEnvelope`, geography/geometry filters |
| Local Environment | Docker Compose                                              |

---

## Architecture

```mermaid
flowchart TB
    Browser["Browser / Mobile Device"]

    subgraph Frontend["Frontend"]
        App["App JS"]
        Leaflet["Leaflet Map"]
        Modal["Accessible Report Modal"]
        Stats["Visible Area Stats"]
    end

    subgraph Backend["Go Backend"]
        Router["HTTP Router"]
        Reports["Report Handler"]
        Places["Quiet Places Handler"]
        Sessions["Anonymous Identity"]
        Limiter["Rate Limiter"]
        Hub["WebSocket Hub"]
    end

    subgraph Database["PostgreSQL + PostGIS"]
        ReportTable["reports"]
        ConfirmTable["confirmations"]
        GeoIndex["Spatial Indexes"]
    end

    Browser --> App
    App --> Leaflet
    App --> Modal
    App --> Stats

    App -->|REST| Router
    App <-->|WebSocket| Hub

    Router --> Reports
    Router --> Places
    Reports --> Sessions
    Reports --> Limiter
    Reports --> ReportTable
    Reports --> ConfirmTable
    Places --> GeoIndex
    Places --> ReportTable

    Reports --> Hub
    Hub --> App
```

---

## How the Map Works

The frontend tracks the current map viewport and sends its geographic bounds to the backend.

For smaller visible areas, the app can combine:

* map center
* radius
* viewport bounds

For larger visible areas, the app switches to a **bounds-only search mode**, avoiding a misleading radius cap that would cut off part of the visible map.

```mermaid
flowchart LR
    A["User clicks Search this area"] --> B["Frontend reads map bounds"]
    B --> C{"Viewport within radius limit?"}
    C -->|Yes| D["Send center + radius + bounds"]
    C -->|No| E["Send bounds-only query"]
    D --> F["PostGIS filters reports"]
    E --> F
    F --> G["Rank quiet places"]
    G --> H["Render results and update stats"]
```

This makes the “Search this area” button behave exactly as users expect: it searches the area they are actually seeing.

---

## Realtime Flow

```mermaid
sequenceDiagram
    participant U as User
    participant F as Frontend
    participant A as Go API
    participant D as PostgreSQL/PostGIS
    participant W as WebSocket Hub
    participant O as Other Clients

    U->>F: Submit silence/noise report
    F->>A: POST /api/reports
    A->>D: Validate and persist report
    D-->>A: Saved report
    A->>W: Publish new_report event
    W-->>O: Broadcast to subscribed clients
    W-->>F: Confirm update
```

When users move or zoom the map, the frontend updates its WebSocket subscription bounds and reloads existing reports for the new visible area.

---

## Geospatial Behavior

Silence Map uses PostGIS to make location-based behavior reliable and explainable.

The backend supports:

| Mode                       | Description                                                       |
| -------------------------- | ----------------------------------------------------------------- |
| Center + radius            | Search near a specific point                                      |
| Bounds-only                | Search the entire visible map area                                |
| Radius + bounds            | Search within a radius while respecting the viewport              |
| Recent reports by viewport | Load reports currently relevant to the visible map                |
| Quiet-place ranking        | Rank areas using quietness, confirmations, recency, and proximity |

The app validates:

* latitude range
* longitude range
* radius limits
* complete bounds
* invalid or partial viewport parameters

---

## Demo Identity

Silence Map does not require users to create an account.

Instead, it uses a lightweight **signed anonymous session** suitable for portfolio and demo environments.

This allows the backend to:

* avoid trusting arbitrary `user_id` values from the client
* prevent obvious self-confirmation
* reject duplicate confirmations
* apply rate limits more consistently
* keep the demo simple and frictionless

This is intentionally not a full authentication system. A production version would use OAuth2, passkeys, or another stronger identity layer.

---

## Accessibility

The interface includes real accessibility considerations:

* keyboard-accessible report creation
* focusable controls
* modal focus trap
* `Escape` key support
* focus restoration after closing the modal
* semantic dialog attributes
* readable controls and status messages
* mobile-friendly touch targets

The report modal follows this interaction model:

```mermaid
flowchart TD
    A["Open report modal"] --> B["Store previously focused element"]
    B --> C["Move focus into modal"]
    C --> D["Trap Tab / Shift+Tab inside modal"]
    D --> E{"User closes modal"}
    E --> F["Restore focus to trigger element"]
```

---

## Responsive Design

Silence Map is designed to work across:

* desktop
* tablet
* mobile portrait
* mobile landscape

The layout accounts for:

* dynamic mobile browser toolbars
* `100dvh`
* safe-area insets
* notches and home indicators
* scrollable mobile panels
* map resizing after UI changes
* touch-friendly controls

---

## Project Structure

```text
.
├── cmd/
│   └── server/                 # Application entrypoint
├── internal/
│   ├── handler/                # HTTP handlers
│   ├── identity/               # Anonymous signed session handling
│   ├── middleware/             # Security, logging, request handling
│   ├── repository/             # PostgreSQL/PostGIS persistence
│   ├── usecase/                # Business rules and validation
│   └── websocket/              # Realtime hub and subscriptions
├── web/
│   ├── index.html              # App shell
│   ├── styles.css              # Responsive UI
│   ├── app.js                  # Frontend behavior
│   ├── app_core.js             # Testable frontend logic
│   └── index.test.js           # Frontend static checks
├── migrations/                 # Database schema
├── docker-compose.yml
├── Dockerfile
├── Makefile
├── .env.example
└── README.md
```

---

## Getting Started

### 1. Clone the repository

```bash
git clone https://github.com/your-username/silence-map.git
cd silence-map
```

### 2. Create the environment file

```bash
cp .env.example .env
```

### 3. Start the app with Docker Compose

```bash
docker compose up --build
```

### 4. Open the app

```text
http://localhost:8080
```

---

## Environment Variables

Example `.env`:

```bash
DATABASE_URL=postgres://postgres:postgres@db:5432/silencemap?sslmode=disable
PORT=8080
SESSION_SECRET=change-me-in-local-development
APP_ENV=development
```

For local shell execution without Docker, export the variables manually.

### Linux/macOS

```bash
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/silencemap?sslmode=disable"
export PORT="8080"
export SESSION_SECRET="change-me-in-local-development"
```

### PowerShell

```powershell
$env:DATABASE_URL="postgres://postgres:postgres@localhost:5432/silencemap?sslmode=disable"
$env:PORT="8080"
$env:SESSION_SECRET="change-me-in-local-development"
```

---

## Running Locally Without Docker

Start PostgreSQL with PostGIS enabled, configure `DATABASE_URL`, then run:

```bash
go run ./cmd/server
```

Then open:

```text
http://localhost:8080
```

---

## Important Local Development Note

Use the app through HTTP:

```text
http://localhost:8080
```

or through a local static server if configured.

Avoid relying on direct `file://` usage for full backend features, because browser cookie and origin behavior can make session identity unreliable in that mode.

---

## API Overview

### Create a report

```http
POST /api/reports
Content-Type: application/json
```

```json
{
  "place_name": "City Library",
  "latitude": -23.55052,
  "longitude": -46.633308,
  "quietness": 92,
  "noise_level": "quiet"
}
```

### List recent reports

```http
GET /api/reports/recent?north=-23.54&south=-23.56&east=-46.62&west=-46.65
```

### Search quiet places

```http
GET /api/places/quiet?lat=-23.55052&lng=-46.633308&radius=2500
```

### Search the current visible map area

```http
GET /api/places/quiet?north=-23.54&south=-23.56&east=-46.62&west=-46.65&radius=0
```

### Confirm a report

```http
POST /api/reports/{id}/confirm
```

---

## WebSocket Events

### Connection

```text
ws://localhost:8080/ws
```

### Subscribe to bounds

```json
{
  "type": "subscribe",
  "bounds": {
    "north": -23.54,
    "south": -23.56,
    "east": -46.62,
    "west": -46.65
  }
}
```

### New report event

```json
{
  "type": "new_report",
  "report": {
    "id": "uuid",
    "place_name": "City Library",
    "latitude": -23.55052,
    "longitude": -46.633308,
    "quietness": 92
  }
}
```

### Confirmation event

```json
{
  "type": "confirmation",
  "report_id": "uuid",
  "confirmations": 3
}
```

---

## Testing

### Backend tests

```bash
go test ./...
```

### Frontend checks

```bash
node web/index.test.js
```

### Full local validation

```bash
make test
```

If available, run the full Docker environment:

```bash
docker compose up --build
```

---

## Manual Demo Script

### 1. Start the system

```bash
docker compose up --build
```

### 2. Open the app

```text
http://localhost:8080
```

### 3. Create a report

Click on the map or use the **Report at map center** button.

Fill in:

* place name
* quietness level
* optional notes

Submit the report.

### 4. Confirm realtime behavior

Open the app in a second browser tab.

Create a report in one tab and watch it appear in the other.

### 5. Search the visible area

Move or zoom the map.

Click:

```text
Search this area
```

The app will search the current viewport instead of relying only on a fixed radius.

### 6. Test mobile layout

Open DevTools and test:

* iPhone-size viewport
* Android-size viewport
* tablet viewport
* landscape orientation

---

## UI Behavior

| Interaction          | Behavior                                             |
| -------------------- | ---------------------------------------------------- |
| Click map            | Opens report modal at clicked location               |
| Report at map center | Opens report modal using current map center          |
| Move or zoom map     | Reloads visible reports and updates WebSocket bounds |
| Search this area     | Searches quiet places inside current viewport        |
| Confirm report       | Optimistically updates UI and rolls back on failure  |
| WebSocket reconnect  | Re-subscribes to the latest visible bounds           |
| Mobile panel toggle  | Resizes map safely after layout change               |

---

## Security Notes

Silence Map includes several safety practices appropriate for a public demo:

* signed anonymous session identity
* request body limits
* strict JSON decoding
* coordinate validation
* radius and bounds validation
* duplicate confirmation protection
* self-confirmation prevention
* rate limiting
* safe DOM rendering with `textContent` / DOM APIs
* no trusted client-side `user_id`
* clean error responses

This is not a replacement for production authentication, abuse detection, or enterprise-grade observability.

---

## Production-Like Qualities

This project demonstrates more than a basic CRUD application.

It includes:

* geospatial querying
* realtime map updates
* viewport-aware search
* anonymous identity
* backend validation
* frontend accessibility
* mobile-first UI behavior
* Dockerized development
* separation between handlers, use cases, repositories, and frontend logic

---

## Known Demo Boundaries

Silence Map is portfolio-ready, but it is still a demo application.

Current boundaries:

* no full user account system
* no OAuth2 or RBAC
* no distributed WebSocket scaling
* no production observability stack
* no CDN-free/offline map tile strategy
* no advanced abuse prevention beyond demo rate limiting
* no native mobile app

These boundaries are intentional to keep the project focused, understandable, and easy to run.

---

## Suggested Screenshots

Add screenshots or GIFs in:

```text
docs/assets/
```

Recommended files:

```text
docs/assets/silence-map-desktop.png
docs/assets/silence-map-mobile.png
docs/assets/report-modal.png
docs/assets/realtime-demo.gif
```

Then include them here:

```md
![Silence Map Desktop](docs/assets/silence-map-desktop.png)
![Silence Map Mobile](docs/assets/silence-map-mobile.png)
```

---

## Roadmap

The project is already suitable for portfolio presentation. Future production-grade extensions could include:

* persistent user accounts
* user reputation scoring
* moderation tools
* historical quietness predictions
* noise heatmaps by hour and day
* offline-first mobile support
* clustering for dense report areas
* Prometheus/Grafana observability
* Playwright end-to-end test suite
* Kubernetes deployment manifests

---

## Why This Project Belongs in a Portfolio

Silence Map demonstrates practical engineering ability across multiple areas:

* backend API design
* geospatial data modeling
* realtime communication
* frontend UX
* accessibility
* security awareness
* responsive design
* database querying
* Docker-based development
* product thinking

It is not just technically interesting.
It solves a clear human problem.

---

## License

MIT License. See [LICENSE](LICENSE) file for details.
