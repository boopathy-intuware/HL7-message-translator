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
- `frontend/` — React + Vite + TypeScript.
- `migrations/` — `golang-migrate` SQL migrations for the Postgres schema (`messages`, `fhir_resources`).
- `docker-compose.yml` — local Postgres 16 instance.

## Getting started

### Database

```bash
docker compose up -d
```

Starts Postgres 16 on `localhost:5432` (db `hl7_message_translator`, user/password `hl7`/`hl7`).

### Migrations

Requires the [`golang-migrate`](https://github.com/golang-migrate/migrate) CLI installed separately.

```bash
migrate -path migrations -database "postgres://hl7:hl7@localhost:5432/hl7_message_translator?sslmode=disable" up
```

### Backend

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

### Frontend

```bash
cd frontend
npm install
npm run dev
```

## Status

The backend ingest pipeline is functional end-to-end for ADT^A01 and ORU^R01 messages: parse → map to FHIR → persist → ACK, plus read endpoints, health/readiness, structured logging, and Prometheus metrics. Not yet built: the frontend ingestion dashboard (still the Vite starter page) and mapping support for other HL7v2 message types. See `CLAUDE.md` for more detail on architecture and conventions.

This README will be updated as the project grows.
