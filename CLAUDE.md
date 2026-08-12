# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

This is a monorepo for an HL7v2-to-FHIR message translator. Current state:

- `backend/` — Go module with a chi-router HTTP server (`cmd/server/main.go`, `/healthz` only) and an `hl7/` package implementing a minimal HL7v2 parser: `Parse(raw string) (*Message, error)` produces a `Message` with `MSH` and `PID` (nilable) fields, backed by table-driven tests in `hl7/parser_test.go` with synthetic fixtures under `hl7/testdata/`. Only the MSH and PID segments are parsed so far — no OBR/OBX (lab result) support, no FHIR mapping, and no ACK generation yet.
- `migrations/` — `golang-migrate`-style migration pair `000001_init_schema.{up,down}.sql` creating `messages` (id, raw_message, message_type, received_at, parse_status, error_detail nullable) and `fhir_resources` (id, message_id fk, resource_type, resource_json jsonb, created_at) tables.
- `frontend/` — scaffolded via `npm create vite@latest -- --template react-ts`; dependencies installed, `npm run build` verified working. Just the Vite starter page so far — no ingestion dashboard or API integration yet.
- `docker-compose.yml` — runs a single `postgres:16` service (db `hl7_message_translator`, user/password `hl7`/`hl7`, port 5432) with a named volume and healthcheck.

**Hard rule:** never use real patient data anywhere in this repo — code, tests, fixtures, seed data, docs. Sample HL7v2 messages must be hand-written/synthetic or from publicly available sample sources.

Update this file as these pieces land so it reflects reality, not the plan.

## Commands

### Backend (Go, module `hl7-message-translator/backend`)

Run all commands from `backend/`:

- Build: `go build ./...`
- Run the server: `go run ./cmd/server` (listens on `:8080`, override with `SERVER_ADDR` env var)
- Run all tests: `go test ./...`
- Run a single test: `go test ./hl7 -run TestName`
- Verbose test output: `go test -v ./...`
- Tidy/verify dependencies after adding imports: `go mod tidy`

### Database (docker-compose)

Run from repo root: `docker compose up -d` starts Postgres 16 on `localhost:5432` (db `hl7_message_translator`, user/password `hl7`/`hl7`).

### Migrations (golang-migrate)

Run from repo root, pointing at the Postgres instance started by docker-compose (`migrate` CLI must be installed separately — it is not vendored in this repo):

- Apply: `migrate -path migrations -database "postgres://hl7:hl7@localhost:5432/hl7_message_translator?sslmode=disable" up`
- Roll back one step: `migrate -path migrations -database "$DATABASE_URL" down 1`
- Create a new migration pair: `migrate create -ext sql -dir migrations -seq <name>`

### Frontend (React + Vite + TypeScript)

Run from `frontend/`: `npm install`, `npm run dev`, `npm run build`.

## Architecture

The system ingests raw HL7v2 messages, parses them, and stores both the raw message and a derived FHIR representation:

- **`backend/hl7`** is the parsing layer. It is implemented as three nested splits: segments split on `\r` (also accepting bare `\n`, since hand-typed test messages often lack proper HL7 line endings), fields within a segment split on `|`, and components within a field split on `^`. The MSH segment is a special case for field indexing — the character immediately after `MSH` is the field separator itself, so the encoding-characters field (`^~\&`) occupies the first split position rather than field 1, shifting every subsequent MSH field index by one relative to other segments (e.g. PID). `parseMSH`/`parsePID` in `parse.go` account for this offset explicitly rather than reusing a generic field-index helper across segment types. `Parse` returns a `*ParseError` (with `Segment` and `Reason`) when the message has no segments, is missing the required MSH segment, or MSH/PID don't carry enough fields — unrecognized segments and missing optional components (e.g. no MSH-9 trigger event) are tolerated rather than treated as errors.
- **Postgres schema** (via `migrations/`) separates the ingestion record from the derived resource: `messages` holds the raw inbound text plus parse status/error, and `fhir_resources` holds one or more derived FHIR resources per message (jsonb, linked by `message_id`). A message that fails to parse is expected to still be persisted in `messages` with `parse_status` reflecting the failure and `error_detail` populated, rather than being dropped.
- **`backend/cmd/server`** is the HTTP entrypoint (chi router). Business logic should live in packages under `backend/` (e.g. `hl7`), not in `cmd/server`, so it stays testable without an HTTP server.
