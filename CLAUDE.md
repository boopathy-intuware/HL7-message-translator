# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

This is a monorepo for an HL7v2-to-FHIR message translator. Current state:

- `backend/` — Go module with a chi-router HTTP server (`cmd/server/main.go`), backed by six packages: `hl7` (parser), `mapper` (HL7→FHIR conversion), `ack` (HL7v2 ACK generation), `store` (Postgres persistence), `metrics` (Prometheus collectors), and `api` (HTTP handlers). `hl7.Parse(raw string) (*Message, error)` produces a `Message` with `MSH`, `PID`, `PV1`, `OBR` (all nilable except MSH), and `OBX` (a slice, one per reported result) fields, backed by table-driven tests in `hl7/parser_test.go` with synthetic fixtures under `hl7/testdata/`. `mapper.MapADTA01` converts PID+PV1 into a `Patient`+`Encounter`; `mapper.MapORUR01` converts PID+OBR+OBX into a `Patient`+`DiagnosticReport`+one `Observation` per OBX, with Encounter/DiagnosticReport/Observation all referencing the Patient by ID (and DiagnosticReport referencing each Observation by ID) the way real FHIR does. Backed by table-driven tests in `mapper/mapper_test.go` asserting exact JSON output against the same `hl7/testdata/` fixtures.
  - **HTTP API**: `POST /api/hl7/messages` ingests a raw HL7v2 message, parses + maps it, persists the raw message and derived FHIR resources, and returns an HL7v2 ACK (`AA` on success, `AE` if parsing or mapping failed). `GET /api/messages` lists ingested messages with status; `GET /api/messages/:id` returns a message plus its FHIR resources (404 if not found). `GET /health` is a liveness probe; `GET /ready` additionally pings Postgres; `GET /healthz` is kept as an alias of `/health`. `GET /metrics` exposes Prometheus counters/histogram (`hl7_messages_ingested_total`, `hl7_parse_failures_total`, `hl7_processing_duration_seconds`), all labeled by `message_type` (e.g. `"ADT^A01"`). Every request is logged as one structured JSON line (method, path, status, latency_ms, plus message_type/parse_status on the ingest route) via the `api.RequestLogger` middleware, built on `log/slog`.
  - **Read-only FHIR API**, modeled loosely on FHIR's RESTful conventions: `GET /fhir/Patient/:id` returns the stored Patient resource JSON directly (a FHIR-style read; 404 if not found). `GET /fhir/Patient?family=:name` and `GET /fhir/Observation?patient=:id` are searches — each returns a minimal FHIR `searchset` Bundle (`{resourceType: "Bundle", type: "searchset", total, entry: [{resource: ...}]}`), and a search with no matches is a `200` with an empty Bundle, never a `404`. Both require their query parameter (`400` if missing).
  - **`backend/api`** (`handlers.go`, `fhir_handlers.go`, `logging.go`) holds the `Handler` type wiring `store.Store` + `metrics.Metrics` + `*slog.Logger` to chi routes. `mapToFHIR` dispatches a parsed message to `mapper.MapADTA01`/`MapORUR01` by its `MSH-9` type^trigger; an unrecognized type or a mapper error (e.g. ADT^A01 with no PV1) is treated the same as an HL7 parse failure — `parse_status` in Postgres means "did this message end up as usable FHIR", not just "did hl7.Parse succeed". Tested in `api/handlers_test.go` and `api/fhir_handlers_test.go` against an in-memory `fakeStore` (`api/fake_store_test.go`) rather than real Postgres.
  - **`backend/ack`** builds the MSH+MSA ACK pair. It recovers the inbound MSH-10 control ID via its own lenient scan of the raw text (`extractControlID`), independent of `hl7.Parse` — that matters because `hl7.Parse` returns a nil `*Message` on any failure, so the ACK still needs a way to echo the control ID when only a later segment (not MSH) caused the parse to fail.
  - **`backend/store`** defines the `Store` interface (`IngestMessage`, `ListMessages`, `GetMessage`, `Ping`, `GetPatientByID`, `SearchPatientsByFamilyName`, `ListObservationsForPatient`) and `PostgresStore`, its `database/sql` + `github.com/lib/pq` implementation. `IngestMessage` writes the message row and all of its FHIR resource rows inside one transaction, so a message never ends up partially persisted. The FHIR read queries all operate on `fhir_resources.resource_json` directly (Postgres `jsonb` operators: `->>'id'` for the Patient-by-id read, `jsonb_array_elements` + `ILIKE` for the family-name search) rather than adding new relational columns — `ListObservationsForPatient` in particular joins Observation and Patient resources through their shared `message_id`, not through the Observation's own embedded `subject.reference`, since a patient can be re-derived across several messages and every one of them needs to contribute its Observations. `GetPatientByID` and the search methods return the most-recently-derived match(es) first when a patient was derived from more than one message.
  - **`backend/metrics`** builds the three Prometheus collectors above via `metrics.New(prometheus.Registerer)`; callers (including tests) must pass a fresh `prometheus.NewRegistry()` rather than the global default, since re-registering the same collector names panics.
- `migrations/` — `golang-migrate`-style migration pair `000001_init_schema.{up,down}.sql` creating `messages` (id, raw_message, message_type, received_at, parse_status, error_detail nullable) and `fhir_resources` (id, message_id fk, resource_type, resource_json jsonb, created_at) tables.
- `frontend/` — scaffolded via `npm create vite@latest -- --template react-ts`; dependencies installed, `npm run build` verified working. Just the Vite starter page so far — no ingestion dashboard or API integration yet.
- `docker-compose.yml` — runs a single `postgres:16` service (db `hl7_message_translator`, user/password `hl7`/`hl7`, port 5432) with a named volume and healthcheck.

**Hard rule:** never use real patient data anywhere in this repo — code, tests, fixtures, seed data, docs. Sample HL7v2 messages must be hand-written/synthetic or from publicly available sample sources.

Update this file as these pieces land so it reflects reality, not the plan.

## Commands

### Backend (Go, module `hl7-message-translator/backend`)

Run all commands from `backend/`:

- Build: `go build ./...`
- Run the server: `go run ./cmd/server` (listens on `:8080`, override with `SERVER_ADDR`; connects to Postgres via `DATABASE_URL`, defaulting to the docker-compose credentials `postgres://hl7:hl7@localhost:5432/hl7_message_translator?sslmode=disable` if unset — the schema must already be applied, see Migrations below)
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

- **`backend/hl7`** is the parsing layer. It is implemented as three nested splits: segments split on `\r` (also accepting bare `\n`, since hand-typed test messages often lack proper HL7 line endings), fields within a segment split on `|`, and components within a field split on `^`. The MSH segment is a special case for field indexing — the character immediately after `MSH` is the field separator itself, so the encoding-characters field (`^~\&`) occupies the first split position rather than field 1, shifting every subsequent MSH field index by one relative to other segments (e.g. PID, PV1, OBR, OBX). `parseMSH`/`parsePID`/`parsePV1`/`parseOBR`/`parseOBX` in `parse.go` account for this offset explicitly rather than reusing a generic field-index helper across segment types. `Parse` returns a `*ParseError` (with `Segment` and `Reason`) when the message has no segments, is missing the required MSH segment, or a recognized segment doesn't carry enough fields to populate the ones this parser reads (e.g. PV1 without a patient class, OBX without a value) — unrecognized segments and missing optional fields/components (e.g. no MSH-9 trigger event, no PV1 admit date) are tolerated rather than treated as errors. OBX repeats once per reported result, so `Message.OBX` is a slice built by appending as each OBX segment is seen.
- **`backend/mapper`** converts a parsed `*hl7.Message` into simplified (not fully spec-compliant) FHIR R4 JSON-shaped Go structs, tagged for `encoding/json`. Resource IDs are synthesized rather than carried by HL7 directly: Encounter/DiagnosticReport use the message's MSH-10 (`MessageControlID`) as their ID, and each Observation's ID is `<MessageControlID>-<OBX-1 set ID>` — this gives DiagnosticReport.result and every resource's Patient reference something stable to point at, since a later read API needs to look up Observations by patient ID.
- **Postgres schema** (via `migrations/`) separates the ingestion record from the derived resource: `messages` holds the raw inbound text plus parse status/error, and `fhir_resources` holds one or more derived FHIR resources per message (jsonb, linked by `message_id`). A message that fails to parse is expected to still be persisted in `messages` with `parse_status` reflecting the failure and `error_detail` populated, rather than being dropped.
- **`backend/cmd/server`** is the HTTP entrypoint (chi router): it opens the `PostgresStore`, builds the Prometheus registry and `api.Handler`, mounts `api.RequestLogger` + chi's `Recoverer`, and registers routes via `Handler.Routes` plus `/metrics` and the `/healthz` alias directly. Business logic should live in packages under `backend/` (e.g. `hl7`, `api`), not in `cmd/server`, so it stays testable without an HTTP server.
