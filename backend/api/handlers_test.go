package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"

	"hl7-message-translator/backend/metrics"
	"hl7-message-translator/backend/store"
)

// Fixtures live under ../hl7/testdata and are shared with the hl7, mapper,
// and ack packages' own tests — they are synthetic/hand-written and must
// never contain real patient data.
func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "hl7", "testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %q: %v", name, err)
	}
	return strings.ReplaceAll(string(data), "\n", "\r")
}

// newTestRouter builds a chi router with every Handler route registered
// against a fresh fakeStore and an isolated Prometheus registry — each
// test gets its own registry since registering the same collector name
// twice against a shared one panics.
func newTestRouter(t *testing.T) (chi.Router, *fakeStore) {
	t.Helper()
	fs := newFakeStore()
	h := NewHandler(fs, metrics.New(prometheus.NewRegistry()), slog.New(slog.NewJSONHandler(io.Discard, nil)))

	r := chi.NewRouter()
	h.Routes(r)
	return r, fs
}

func post(r chi.Router, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func get(r chi.Router, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestIngestHL7_SuccessfulADT(t *testing.T) {
	r, fs := newTestRouter(t)
	raw := readFixture(t, "adt_a01_valid.hl7")

	rec := post(r, "/api/hl7/messages", raw)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	ackBody := rec.Body.String()
	if !strings.Contains(ackBody, "MSA|AA|MSG00001") {
		t.Errorf("ACK body = %q, want it to contain MSA|AA|MSG00001", ackBody)
	}

	if len(fs.messages) != 1 {
		t.Fatalf("stored %d messages, want 1", len(fs.messages))
	}
	got := fs.messages[0]
	if got.ParseStatus != store.ParseStatusSuccess {
		t.Errorf("ParseStatus = %q, want %q", got.ParseStatus, store.ParseStatusSuccess)
	}
	if got.MessageType != "ADT^A01" {
		t.Errorf("MessageType = %q, want ADT^A01", got.MessageType)
	}
	if got.ErrorDetail != nil {
		t.Errorf("ErrorDetail = %q, want nil on success", *got.ErrorDetail)
	}
	if got.RawMessage != raw {
		t.Errorf("stored RawMessage does not match the ingested body")
	}

	resources := fs.resources[got.ID]
	if len(resources) != 2 {
		t.Fatalf("stored %d FHIR resources, want 2 (Patient, Encounter)", len(resources))
	}
	if resources[0].ResourceType != "Patient" || resources[1].ResourceType != "Encounter" {
		t.Errorf("resource types = [%s, %s], want [Patient, Encounter]", resources[0].ResourceType, resources[1].ResourceType)
	}
}

func TestIngestHL7_SuccessfulORU(t *testing.T) {
	r, fs := newTestRouter(t)
	raw := readFixture(t, "oru_r01_valid.hl7")

	rec := post(r, "/api/hl7/messages", raw)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "MSA|AA|MSG00002") {
		t.Errorf("ACK body = %q, want it to contain MSA|AA|MSG00002", rec.Body.String())
	}

	resources := fs.resources[fs.messages[0].ID]
	// Patient + DiagnosticReport + one Observation per OBX (3 in the fixture).
	if len(resources) != 5 {
		t.Fatalf("stored %d FHIR resources, want 5 (Patient, DiagnosticReport, 3 Observations)", len(resources))
	}
}

func TestIngestHL7_ParseFailure(t *testing.T) {
	r, fs := newTestRouter(t)
	raw := readFixture(t, "missing_msh.hl7")

	rec := post(r, "/api/hl7/messages", raw)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (an ingest request that stores an AE ACK is still a successful HTTP call); body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "MSA|AE|") {
		t.Errorf("ACK body = %q, want it to contain MSA|AE|", rec.Body.String())
	}

	if len(fs.messages) != 1 {
		t.Fatalf("stored %d messages, want 1 (a message that fails to parse must still be persisted)", len(fs.messages))
	}
	got := fs.messages[0]
	if got.ParseStatus != store.ParseStatusFailed {
		t.Errorf("ParseStatus = %q, want %q", got.ParseStatus, store.ParseStatusFailed)
	}
	if got.ErrorDetail == nil || *got.ErrorDetail == "" {
		t.Errorf("ErrorDetail = %v, want a populated parse error", got.ErrorDetail)
	}
	if len(fs.resources[got.ID]) != 0 {
		t.Errorf("stored %d FHIR resources for a message that failed to parse, want 0", len(fs.resources[got.ID]))
	}
}

func TestIngestHL7_MappingFailure_MissingPV1(t *testing.T) {
	r, fs := newTestRouter(t)
	// Structurally valid MSH+PID (hl7.Parse succeeds), but ADT^A01 mapping
	// requires a PV1 segment that this message doesn't carry.
	raw := "MSH|^~\\&|REG_SYS|SYNTH_HOSPITAL|ADT_SYS|SYNTH_HOSPITAL|20260812093000||ADT^A01|MSG00099|P|2.3\r" +
		"PID|1||123459^^^MRN||TEST^PATIENT||19850101|M\r"

	rec := post(r, "/api/hl7/messages", raw)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "MSA|AE|MSG00099") {
		t.Errorf("ACK body = %q, want it to contain MSA|AE|MSG00099 (control ID still recoverable even though mapping failed)", rec.Body.String())
	}

	got := fs.messages[0]
	if got.ParseStatus != store.ParseStatusFailed {
		t.Errorf("ParseStatus = %q, want %q (HL7 parsed fine, but FHIR mapping failed)", got.ParseStatus, store.ParseStatusFailed)
	}
	if got.ErrorDetail == nil || !strings.Contains(*got.ErrorDetail, "PV1") {
		t.Errorf("ErrorDetail = %v, want it to mention the missing PV1 segment", got.ErrorDetail)
	}
	if len(fs.resources[got.ID]) != 0 {
		t.Errorf("stored %d FHIR resources for a message that failed to map, want 0", len(fs.resources[got.ID]))
	}
}

func TestIngestHL7_UnsupportedMessageType(t *testing.T) {
	r, fs := newTestRouter(t)
	raw := "MSH|^~\\&|REG_SYS|SYNTH_HOSPITAL|ADT_SYS|SYNTH_HOSPITAL|20260812093000||ADT^A08|MSG00100|P|2.3\r" +
		"PID|1||123459^^^MRN||TEST^PATIENT||19850101|M\r"

	rec := post(r, "/api/hl7/messages", raw)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "MSA|AE|MSG00100") {
		t.Errorf("ACK body = %q, want it to contain MSA|AE|MSG00100", rec.Body.String())
	}
	got := fs.messages[0]
	if got.MessageType != "ADT^A08" {
		t.Errorf("MessageType = %q, want ADT^A08", got.MessageType)
	}
	if got.ParseStatus != store.ParseStatusFailed {
		t.Errorf("ParseStatus = %q, want %q", got.ParseStatus, store.ParseStatusFailed)
	}
}

func TestListAndGetMessages(t *testing.T) {
	r, _ := newTestRouter(t)
	post(r, "/api/hl7/messages", readFixture(t, "adt_a01_valid.hl7"))
	post(r, "/api/hl7/messages", readFixture(t, "oru_r01_valid.hl7"))

	listRec := get(r, "/api/messages")
	if listRec.Code != http.StatusOK {
		t.Fatalf("GET /api/messages status = %d, want 200", listRec.Code)
	}
	var summaries []store.MessageSummary
	if err := json.Unmarshal(listRec.Body.Bytes(), &summaries); err != nil {
		t.Fatalf("decoding list response: %v; body: %s", err, listRec.Body.String())
	}
	if len(summaries) != 2 {
		t.Fatalf("got %d messages, want 2", len(summaries))
	}
	// ListMessages returns most-recently-ingested first.
	if summaries[0].MessageType != "ORU^R01" || summaries[1].MessageType != "ADT^A01" {
		t.Errorf("message order = [%s, %s], want [ORU^R01, ADT^A01]", summaries[0].MessageType, summaries[1].MessageType)
	}

	getRec := get(r, "/api/messages/1")
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /api/messages/1 status = %d, want 200; body: %s", getRec.Code, getRec.Body.String())
	}
	var detail messageDetail
	if err := json.Unmarshal(getRec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decoding detail response: %v; body: %s", err, getRec.Body.String())
	}
	if detail.ID != 1 || detail.MessageType != "ADT^A01" {
		t.Errorf("detail = %+v, want ID 1 and MessageType ADT^A01", detail)
	}
	if len(detail.Resources) != 2 {
		t.Errorf("detail has %d FHIR resources, want 2", len(detail.Resources))
	}
}

func TestGetMessage_NotFound(t *testing.T) {
	r, _ := newTestRouter(t)
	rec := get(r, "/api/messages/999")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetMessage_InvalidID(t *testing.T) {
	r, _ := newTestRouter(t)
	rec := get(r, "/api/messages/not-a-number")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHealth(t *testing.T) {
	r, _ := newTestRouter(t)
	rec := get(r, "/health")
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestReady(t *testing.T) {
	r, fs := newTestRouter(t)

	if rec := get(r, "/ready"); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d when the database is reachable", rec.Code, http.StatusOK)
	}

	fs.pingErr = errors.New("database unavailable")
	if rec := get(r, "/ready"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d when the database ping fails", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestRequestLogger_CapturesIngestFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	fs := newFakeStore()
	h := NewHandler(fs, metrics.New(prometheus.NewRegistry()), logger)

	r := chi.NewRouter()
	r.Use(RequestLogger(logger))
	h.Routes(r)

	post(r, "/api/hl7/messages", readFixture(t, "adt_a01_valid.hl7"))

	var logLine map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &logLine); err != nil {
		t.Fatalf("log output isn't valid JSON: %v; output: %s", err, buf.String())
	}
	if logLine["message_type"] != "ADT^A01" {
		t.Errorf("log message_type = %v, want ADT^A01", logLine["message_type"])
	}
	if logLine["parse_status"] != store.ParseStatusSuccess {
		t.Errorf("log parse_status = %v, want %v", logLine["parse_status"], store.ParseStatusSuccess)
	}
	if _, ok := logLine["latency_ms"]; !ok {
		t.Errorf("log line missing latency_ms: %v", logLine)
	}
	if logLine["status"] != float64(http.StatusOK) {
		t.Errorf("log status = %v, want %v", logLine["status"], http.StatusOK)
	}
}

// TestIngestHL7_ConcurrentIngestion_ResolvableByRawText ingests many
// distinct messages concurrently and, for each one, resolves its message
// ID the same way the frontend does after a redirect-worthy ingest: list
// GET /api/messages (most-recently-received first) and walk it, fetching
// each candidate via GET /api/messages/:id until one's raw_message
// exactly matches what was just submitted. The ingest endpoint itself
// never reveals the new ID, so this is the only channel available — it
// must stay correct even when many ingests race each other, which is
// what this test guards.
func TestIngestHL7_ConcurrentIngestion_ResolvableByRawText(t *testing.T) {
	r, _ := newTestRouter(t)

	const n = 20
	results := make([]struct {
		raw        string
		resolvedID int64
		resolveErr error
	}, n)

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			raw := fmt.Sprintf(
				"MSH|^~\\&|REG_SYS|SYNTH_HOSPITAL|ADT_SYS|SYNTH_HOSPITAL|20260812093000||ADT^A01|MSG%05d|P|2.3\r"+
					"PID|1||PATID%05d^^^MRN||TEST^PATIENT||19850101|M\r"+
					"PV1|1|I|WARD^101^A\r",
				i, i,
			)
			results[i].raw = raw

			ackRec := post(r, "/api/hl7/messages", raw)
			if ackRec.Code != http.StatusOK {
				results[i].resolveErr = fmt.Errorf("ingest status = %d, body: %s", ackRec.Code, ackRec.Body.String())
				return
			}

			listRec := get(r, "/api/messages")
			if listRec.Code != http.StatusOK {
				results[i].resolveErr = fmt.Errorf("list status = %d", listRec.Code)
				return
			}
			var summaries []store.MessageSummary
			if err := json.Unmarshal(listRec.Body.Bytes(), &summaries); err != nil {
				results[i].resolveErr = fmt.Errorf("decoding list: %w", err)
				return
			}

			for _, summary := range summaries {
				detailRec := get(r, "/api/messages/"+strconv.FormatInt(summary.ID, 10))
				if detailRec.Code != http.StatusOK {
					continue
				}
				var detail messageDetail
				if err := json.Unmarshal(detailRec.Body.Bytes(), &detail); err != nil {
					continue
				}
				if detail.RawMessage == raw {
					results[i].resolvedID = detail.ID
					return
				}
			}
			results[i].resolveErr = fmt.Errorf("no message in the list matched the raw text this goroutine ingested")
		}(i)
	}
	wg.Wait()

	seen := make(map[int64]bool, n)
	for i, res := range results {
		if res.resolveErr != nil {
			t.Fatalf("goroutine %d: %v", i, res.resolveErr)
		}
		if res.resolvedID == 0 {
			t.Fatalf("goroutine %d: resolved to id 0", i)
		}
		if seen[res.resolvedID] {
			t.Fatalf("message id %d was resolved by more than one concurrent request, want each goroutine to resolve a distinct message", res.resolvedID)
		}
		seen[res.resolvedID] = true
	}
	if len(seen) != n {
		t.Fatalf("resolved %d distinct message ids, want %d", len(seen), n)
	}
}
