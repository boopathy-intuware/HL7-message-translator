// Package api implements the HTTP handlers for ingesting HL7v2 messages
// and reading back their stored FHIR resources.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"hl7-message-translator/backend/ack"
	"hl7-message-translator/backend/hl7"
	"hl7-message-translator/backend/mapper"
	"hl7-message-translator/backend/metrics"
	"hl7-message-translator/backend/store"
)

// tracer creates the child spans IngestHL7 uses to break down a request's
// hl7_processing_duration_seconds into its parse/map/persist stages. It
// resolves against whatever TracerProvider is globally registered (a
// no-op unless telemetry.Setup installed a real one), so handler tests
// that don't wire up telemetry still work unchanged.
var tracer = otel.Tracer("hl7-message-translator/backend/api")

// Handler wires the HTTP layer to the persistence and metrics layers.
type Handler struct {
	Store   store.Store
	Metrics *metrics.Metrics
	Logger  *slog.Logger
}

// NewHandler builds a Handler. logger must not be nil; pass slog.Default()
// if the caller has no specific preference.
func NewHandler(s store.Store, m *metrics.Metrics, logger *slog.Logger) *Handler {
	return &Handler{Store: s, Metrics: m, Logger: logger}
}

// Routes registers every handler on r, other than /metrics — which is
// mounted separately since it serves the Prometheus registry directly
// rather than going through Handler.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/api/hl7/messages", h.IngestHL7)
	r.Get("/api/messages", h.ListMessages)
	r.Get("/api/messages/{id}", h.GetMessage)
	r.Get("/health", h.Health)
	r.Get("/ready", h.Ready)

	r.Get("/fhir/Patient/{id}", h.GetPatient)
	r.Get("/fhir/Patient", h.SearchPatients)
	r.Get("/fhir/Observation", h.ListObservations)
}

// IngestHL7 handles POST /api/hl7/messages: it parses the raw HL7v2 body,
// maps it to FHIR resources, persists the raw message and resources
// (marking parse_status success or failed), and responds with an HL7v2 ACK
// — AA on success, AE if either the HL7 parse or the FHIR mapping failed.
func (h *Handler) IngestHL7(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "reading request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	raw := string(body)

	_, parseSpan := tracer.Start(ctx, "hl7.parse")
	msg, parseErr := hl7.Parse(raw)
	if parseErr != nil {
		parseSpan.RecordError(parseErr)
		parseSpan.SetStatus(codes.Error, parseErr.Error())
	}
	parseSpan.End()

	var (
		resources  []namedResource
		mappingErr error
	)
	if parseErr == nil {
		_, mapSpan := tracer.Start(ctx, "fhir.map")
		resources, mappingErr = mapToFHIR(msg)
		if mappingErr != nil {
			mapSpan.RecordError(mappingErr)
			mapSpan.SetStatus(codes.Error, mappingErr.Error())
		}
		mapSpan.End()
	}

	messageType := messageTypeLabel(msg)
	parseStatus := store.ParseStatusSuccess
	var errorDetail *string
	switch {
	case parseErr != nil:
		parseStatus = store.ParseStatusFailed
		detail := parseErr.Error()
		errorDetail = &detail
	case mappingErr != nil:
		parseStatus = store.ParseStatusFailed
		detail := mappingErr.Error()
		errorDetail = &detail
	}

	newResources := make([]store.NewFHIRResource, len(resources))
	for i, res := range resources {
		newResources[i] = store.NewFHIRResource{ResourceType: res.resourceType, ResourceJSON: res.json}
	}

	persistCtx, persistSpan := tracer.Start(ctx, "store.ingest_message")
	persistSpan.SetAttributes(attribute.String("message_type", messageType))
	_, err = h.Store.IngestMessage(persistCtx, store.NewMessage{
		RawMessage:  raw,
		MessageType: messageType,
		ParseStatus: parseStatus,
		ErrorDetail: errorDetail,
	}, newResources)
	if err != nil {
		persistSpan.RecordError(err)
		persistSpan.SetStatus(codes.Error, err.Error())
		persistSpan.End()
		h.Logger.ErrorContext(ctx, "failed to persist ingested message", "error", err)
		http.Error(w, "failed to store message", http.StatusInternalServerError)
		return
	}
	persistSpan.End()

	h.Metrics.MessagesIngested.WithLabelValues(messageType).Inc()
	if parseStatus == store.ParseStatusFailed {
		h.Metrics.ParseFailures.WithLabelValues(messageType).Inc()
	}
	h.Metrics.ProcessingDuration.WithLabelValues(messageType).Observe(time.Since(start).Seconds())
	setRequestFields(ctx, messageType, parseStatus)

	ackMsg := ack.Generate(raw, msg, parseErr, mappingErr, time.Now())
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(ackMsg))
}

// ListMessages handles GET /api/messages.
func (h *Handler) ListMessages(w http.ResponseWriter, r *http.Request) {
	messages, err := h.Store.ListMessages(r.Context())
	if err != nil {
		h.Logger.ErrorContext(r.Context(), "failed to list messages", "error", err)
		http.Error(w, "failed to list messages", http.StatusInternalServerError)
		return
	}
	if messages == nil {
		messages = []store.MessageSummary{}
	}
	respondJSON(w, http.StatusOK, messages)
}

// messageDetail is the response body for GET /api/messages/:id.
type messageDetail struct {
	store.Message
	Resources []store.FHIRResource `json:"fhir_resources"`
}

// GetMessage handles GET /api/messages/:id.
func (h *Handler) GetMessage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid message id", http.StatusBadRequest)
		return
	}

	msg, resources, err := h.Store.GetMessage(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "message not found", http.StatusNotFound)
		return
	}
	if err != nil {
		h.Logger.ErrorContext(r.Context(), "failed to get message", "error", err, "id", id)
		http.Error(w, "failed to get message", http.StatusInternalServerError)
		return
	}
	if resources == nil {
		resources = []store.FHIRResource{}
	}

	respondJSON(w, http.StatusOK, messageDetail{Message: *msg, Resources: resources})
}

// Health handles GET /health: a liveness probe that reports the process is
// up, without checking any dependency.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// Ready handles GET /ready: a readiness probe that additionally checks the
// database is reachable.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.Ping(r.Context()); err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready", "error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ready"))
}

func respondJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

// namedResource pairs a FHIR resource's JSON with its resourceType, ready
// to persist as a store.NewFHIRResource.
type namedResource struct {
	resourceType string
	json         json.RawMessage
}

// messageTypeLabel returns the MSH-9 style "<type>^<trigger>" label used
// for the message_type column and for the message_type metric/log label.
// msg is nil when hl7.Parse failed outright (e.g. no MSH segment at all).
func messageTypeLabel(msg *hl7.Message) string {
	if msg == nil || msg.MSH.MessageType == "" {
		return "UNKNOWN"
	}
	if msg.MSH.TriggerEvent == "" {
		return msg.MSH.MessageType
	}
	return msg.MSH.MessageType + "^" + msg.MSH.TriggerEvent
}

// mapToFHIR dispatches a successfully-parsed message to the mapper
// function for its message type, and marshals the result into named FHIR
// resources ready to persist. It returns an error for any message type
// this translator doesn't yet know how to map (or one the mapper package
// itself rejects, e.g. an ADT^A01 with no PV1 segment).
func mapToFHIR(msg *hl7.Message) ([]namedResource, error) {
	switch messageTypeLabel(msg) {
	case "ADT^A01":
		patient, encounter, err := mapper.MapADTA01(msg)
		if err != nil {
			return nil, err
		}
		return marshalResources(
			resourceOf("Patient", patient),
			resourceOf("Encounter", encounter),
		)
	case "ORU^R01":
		patient, report, observations, err := mapper.MapORUR01(msg)
		if err != nil {
			return nil, err
		}
		pairs := []resourcePair{
			resourceOf("Patient", patient),
			resourceOf("DiagnosticReport", report),
		}
		for i := range observations {
			pairs = append(pairs, resourceOf("Observation", &observations[i]))
		}
		return marshalResources(pairs...)
	default:
		return nil, fmt.Errorf("api: unsupported message type %q for FHIR mapping", messageTypeLabel(msg))
	}
}

// resourcePair is an intermediate value passed to marshalResources — the
// resource's type name alongside the not-yet-marshaled Go value.
type resourcePair struct {
	resourceType string
	value        any
}

func resourceOf(resourceType string, value any) resourcePair {
	return resourcePair{resourceType: resourceType, value: value}
}

func marshalResources(pairs ...resourcePair) ([]namedResource, error) {
	resources := make([]namedResource, len(pairs))
	for i, p := range pairs {
		b, err := json.Marshal(p.value)
		if err != nil {
			return nil, fmt.Errorf("api: marshaling %s: %w", p.resourceType, err)
		}
		resources[i] = namedResource{resourceType: p.resourceType, json: b}
	}
	return resources, nil
}
