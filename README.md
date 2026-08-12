# HL7 Message Translator

A monorepo for an HL7v2-to-FHIR message translator: ingests raw HL7v2 messages (the pipe-delimited format used for hospital admissions and lab result feeds), parses them, and stores both the raw message and a derived FHIR representation.

> **No real patient data.** Every sample HL7v2 message in this repo (tests, fixtures, docs) is hand-written/synthetic or pulled from publicly available sample sources. Never commit real PHI.

## Structure

- `backend/` — Go module (chi router). `backend/hl7` is the HL7v2 parsing layer; `backend/cmd/server` is the HTTP entrypoint.
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
go run ./cmd/server   # listens on :8080, override with SERVER_ADDR
go test ./...
```

### Frontend

```bash
cd frontend
npm install
npm run dev
```

## Status

Early scaffolding. The HL7v2 parser currently handles MSH and PID segments only (ADT^A01 and ORU^R01 message types); FHIR conversion, ACK generation, the ingestion dashboard, and the read API are not yet built. See `CLAUDE.md` for more detail on architecture and conventions.

This README will be updated as the project grows.
