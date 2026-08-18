package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"hl7-message-translator/backend/store"
)

// GetPatient handles GET /fhir/Patient/:id — a FHIR-style read that
// returns the stored Patient resource JSON directly (not wrapped in a
// Bundle), matching real FHIR read semantics.
func (h *Handler) GetPatient(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	resource, err := h.Store.GetPatientByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "Patient not found", http.StatusNotFound)
		return
	}
	if err != nil {
		h.Logger.ErrorContext(r.Context(), "failed to get patient", "error", err, "id", id)
		http.Error(w, "failed to get patient", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(resource.ResourceJSON)
}

// SearchPatients handles GET /fhir/Patient?family=:name — a simple
// case-insensitive substring search over Patient.name.family, returning a
// FHIR searchset Bundle (possibly empty; a search never 404s).
func (h *Handler) SearchPatients(w http.ResponseWriter, r *http.Request) {
	family := r.URL.Query().Get("family")
	if family == "" {
		http.Error(w, "family query parameter is required", http.StatusBadRequest)
		return
	}

	resources, err := h.Store.SearchPatientsByFamilyName(r.Context(), family)
	if err != nil {
		h.Logger.ErrorContext(r.Context(), "failed to search patients", "error", err, "family", family)
		http.Error(w, "failed to search patients", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, newSearchBundle(resources))
}

// ListObservations handles GET /fhir/Observation?patient=:id — every
// Observation resource derived from a message that also derived a Patient
// resource with that id (i.e. linked via their shared source message, not
// via the Observation's own subject reference). Returns a FHIR searchset
// Bundle (possibly empty; a search never 404s, even for an unknown patient
// id).
func (h *Handler) ListObservations(w http.ResponseWriter, r *http.Request) {
	patientID := r.URL.Query().Get("patient")
	if patientID == "" {
		http.Error(w, "patient query parameter is required", http.StatusBadRequest)
		return
	}

	resources, err := h.Store.ListObservationsForPatient(r.Context(), patientID)
	if err != nil {
		h.Logger.ErrorContext(r.Context(), "failed to list observations", "error", err, "patient", patientID)
		http.Error(w, "failed to list observations", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, newSearchBundle(resources))
}

// searchBundle is a minimal FHIR searchset Bundle: the shape a real FHIR
// server's search endpoints wrap their results in, trimmed to just the
// fields a client needs to enumerate matches.
type searchBundle struct {
	ResourceType string        `json:"resourceType"`
	Type         string        `json:"type"`
	Total        int           `json:"total"`
	Entry        []bundleEntry `json:"entry"`
}

type bundleEntry struct {
	Resource json.RawMessage `json:"resource"`
}

func newSearchBundle(resources []store.FHIRResource) searchBundle {
	entries := make([]bundleEntry, len(resources))
	for i, res := range resources {
		entries[i] = bundleEntry{Resource: res.ResourceJSON}
	}
	return searchBundle{
		ResourceType: "Bundle",
		Type:         "searchset",
		Total:        len(entries),
		Entry:        entries,
	}
}
