# HL7 Message Translator

A monorepo for an HL7v2-to-FHIR message translator: ingests raw HL7v2 messages (the pipe-delimited format used for hospital admissions and lab result feeds), parses them, and stores both the raw message and a derived FHIR representation.

> **No real patient data.** Every sample HL7v2 message in this repo (tests, fixtures, docs) is hand-written/synthetic or pulled from publicly available sample sources. Never commit real PHI.

## Structure

- `backend/` — Go module (chi router):
  - `hl7` — HL7v2 parsing layer
  - `mapper` — HL7v2 → simplified FHIR R4 conversion
  - `ack` — HL7v2 ACK (MSH+MSA) generation
  - `store` — Postgres persistence (`messages` + `fhir_resources`)
  - `metrics` — Prometheus collectors
  - `api` — HTTP handlers and structured request logging
  - `cmd/server` — HTTP entrypoint wiring the above together
- `frontend/` — React + Vite + TypeScript. An ingestion dashboard: an inbox list (`GET /api/messages`) linking to a per-message detail view (`GET /api/messages/:id`) that shows the raw HL7 next to the generated FHIR JSON.
- `migrations/` — `golang-migrate` SQL migrations for the Postgres schema (`messages`, `fhir_resources`).
- `docker-compose.yml` — the full local stack: Postgres 16, a one-shot migration runner, the Go backend, and the frontend dev server.

## Getting started

### Quickstart: everything via docker-compose

```bash
docker compose up -d --build
```

Builds and starts the whole stack in one command:

| Service    | URL                     | Notes                                              |
|------------|-------------------------|-----------------------------------------------------|
| `postgres` | `localhost:5432`        | db `hl7_message_translator`, user/password `hl7`/`hl7` |
| `migrate`  | —                       | applies `migrations/`, then exits                  |
| `backend`  | `localhost:8080`        | waits for `migrate` to finish first                |
| `frontend` | `localhost:5173`        | Vite dev server, live-reloads against the bind-mounted `frontend/` source |

Open `http://localhost:5173` for the inbox. Re-run `docker compose up -d --build` after changing a Dockerfile, `go.mod`, or `package.json` so the image rebuilds.

### Cleanup / tearing down

```bash
docker compose down          # stop + remove containers and the network; keeps the postgres_data volume (data survives)
docker compose down -v       # same, but also delete the postgres_data volume (wipes the database)
docker compose down --rmi local  # also remove the images this repo built (backend/frontend), forcing a full rebuild next time
```

To "eject" back to running things manually on the host (e.g. for faster backend iteration or debugging): stop just the containerized backend/frontend and keep Postgres in Docker —

```bash
docker compose stop backend frontend
```

— then follow the **Backend** and **Frontend** sections below to run them natively against the same Postgres instance (`localhost:5432`). `docker compose up -d backend frontend` brings the containers back.

### Manual setup (without docker-compose for backend/frontend)

Useful for local iteration with your own toolchain instead of the containers above.

#### Database

```bash
docker compose up -d postgres
```

Starts just Postgres 16 on `localhost:5432` (db `hl7_message_translator`, user/password `hl7`/`hl7`).

#### Migrations

Requires the [`golang-migrate`](https://github.com/golang-migrate/migrate) CLI installed separately (the `migrate` compose service above only runs inside the full-stack quickstart).

```bash
migrate -path migrations -database "postgres://hl7:hl7@localhost:5432/hl7_message_translator?sslmode=disable" up
```

#### Backend

```bash
cd backend
go build ./...
go run ./cmd/server   # listens on :8080 (SERVER_ADDR), connects via DATABASE_URL
go test ./...
```

`DATABASE_URL` defaults to the docker-compose credentials (`postgres://hl7:hl7@localhost:5432/hl7_message_translator?sslmode=disable`) if unset — start Postgres and apply migrations first.

### API

- `POST /api/hl7/messages` — ingest a raw HL7v2 message (request body); parses it, maps it to FHIR, persists both, and returns an HL7v2 ACK (`AA` on success, `AE` on parse/mapping failure).
- `GET /api/messages` — list ingested messages with status.
- `GET /api/messages/:id` — a message plus its derived FHIR resources.
- `GET /health` / `GET /ready` — liveness / readiness (readiness also pings Postgres).
- `GET /metrics` — Prometheus metrics (messages ingested, parse failures, processing latency, all by message type).

Read-only, modeled loosely on FHIR's RESTful conventions — searches return a FHIR `searchset` Bundle (empty, not a 404, when nothing matches):

- `GET /fhir/Patient/:id` — a stored Patient resource by its FHIR id (404 if not found).
- `GET /fhir/Patient?family=:name` — patients whose family name contains `:name` (case-insensitive).
- `GET /fhir/Observation?patient=:id` — every Observation derived from a message that also derived a Patient with that id.

#### Frontend

```bash
cd frontend
npm install
npm run dev     # proxies /api to BACKEND_URL, default http://localhost:8080
npm run build
npm test        # vitest component tests (MessageList, MessageDetailView)
```

## Status

The backend ingest pipeline is functional end-to-end for ADT^A01 and ORU^R01 messages: parse → map to FHIR → persist → ACK, plus read endpoints, health/readiness, structured logging, and Prometheus metrics. The frontend has an inbox dashboard (list + raw-vs-FHIR detail view) with component tests. Not yet built: mapping support for other HL7v2 message types, and a UI for ingesting messages (currently `POST /api/hl7/messages` only, via curl/Postman/etc.). See `CLAUDE.md` for more detail on architecture and conventions.

This README will be updated as the project grows.
