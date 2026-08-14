package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestGetPatient_Found(t *testing.T) {
	r, _ := newTestRouter(t)
	post(r, "/api/hl7/messages", readFixture(t, "adt_a01_valid.hl7"))

	rec := get(r, "/fhir/Patient/123456")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var patient struct {
		ResourceType string `json:"resourceType"`
		ID           string `json:"id"`
		Name         []struct {
			Family string `json:"family"`
			Given  string `json:"given"`
		} `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &patient); err != nil {
		t.Fatalf("decoding response: %v; body: %s", err, rec.Body.String())
	}
	if patient.ResourceType != "Patient" || patient.ID != "123456" {
		t.Errorf("got resourceType=%q id=%q, want Patient/123456", patient.ResourceType, patient.ID)
	}
	if len(patient.Name) != 1 || patient.Name[0].Family != "DOE" {
		t.Errorf("got name = %+v, want family DOE", patient.Name)
	}
}

func TestGetPatient_NotFound(t *testing.T) {
	r, _ := newTestRouter(t)
	post(r, "/api/hl7/messages", readFixture(t, "adt_a01_valid.hl7"))

	rec := get(r, "/fhir/Patient/does-not-exist")

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func decodeBundle(t *testing.T, body []byte) searchBundle {
	t.Helper()
	var b searchBundle
	if err := json.Unmarshal(body, &b); err != nil {
		t.Fatalf("decoding search bundle: %v; body: %s", err, body)
	}
	return b
}

func TestSearchPatients_Found(t *testing.T) {
	r, _ := newTestRouter(t)
	// Both fixtures use family name DOE, with different given names and
	// patient IDs — a realistic case for a family-name search to match
	// more than one patient.
	post(r, "/api/hl7/messages", readFixture(t, "adt_a01_valid.hl7")) // DOE^JOHN, id 123456
	post(r, "/api/hl7/messages", readFixture(t, "oru_r01_valid.hl7")) // DOE^JANE, id 123457

	rec := get(r, "/fhir/Patient?family=DOE")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	bundle := decodeBundle(t, rec.Body.Bytes())
	if bundle.ResourceType != "Bundle" || bundle.Type != "searchset" {
		t.Errorf("got resourceType=%q type=%q, want Bundle/searchset", bundle.ResourceType, bundle.Type)
	}
	if bundle.Total != 2 || len(bundle.Entry) != 2 {
		t.Errorf("got %d results, want 2 (one per DOE patient)", bundle.Total)
	}
}

func TestSearchPatients_CaseInsensitiveSubstring(t *testing.T) {
	r, _ := newTestRouter(t)
	post(r, "/api/hl7/messages", readFixture(t, "adt_a01_valid.hl7")) // DOE^JOHN

	rec := get(r, "/fhir/Patient?family=oe") // lowercase substring of "DOE"

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if bundle := decodeBundle(t, rec.Body.Bytes()); bundle.Total != 1 {
		t.Errorf("got %d results, want 1 for a case-insensitive substring match", bundle.Total)
	}
}

func TestSearchPatients_EmptyResult(t *testing.T) {
	r, _ := newTestRouter(t)
	post(r, "/api/hl7/messages", readFixture(t, "adt_a01_valid.hl7")) // DOE^JOHN

	rec := get(r, "/fhir/Patient?family=NOSUCHNAME")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (a search with no matches is still a successful empty result, not 404); body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	bundle := decodeBundle(t, rec.Body.Bytes())
	if bundle.Total != 0 || len(bundle.Entry) != 0 {
		t.Errorf("got %d results, want 0", bundle.Total)
	}
}

func TestSearchPatients_MissingFamilyParam(t *testing.T) {
	r, _ := newTestRouter(t)
	rec := get(r, "/fhir/Patient")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestListObservations_Found(t *testing.T) {
	r, _ := newTestRouter(t)
	post(r, "/api/hl7/messages", readFixture(t, "oru_r01_valid.hl7")) // patient 123457, 3 OBX

	rec := get(r, "/fhir/Observation?patient=123457")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	bundle := decodeBundle(t, rec.Body.Bytes())
	if bundle.Total != 3 {
		t.Errorf("got %d observations, want 3 (one per OBX in the fixture)", bundle.Total)
	}
	var obs struct {
		ResourceType string `json:"resourceType"`
	}
	if err := json.Unmarshal(bundle.Entry[0].Resource, &obs); err != nil {
		t.Fatalf("decoding bundle entry: %v", err)
	}
	if obs.ResourceType != "Observation" {
		t.Errorf("got resourceType = %q, want Observation", obs.ResourceType)
	}
}

func TestListObservations_EmptyResult_PatientHasNoObservations(t *testing.T) {
	r, _ := newTestRouter(t)
	// ADT^A01 maps to Patient+Encounter only — this patient has no
	// Observations, but does exist.
	post(r, "/api/hl7/messages", readFixture(t, "adt_a01_valid.hl7")) // patient 123456

	rec := get(r, "/fhir/Observation?patient=123456")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	bundle := decodeBundle(t, rec.Body.Bytes())
	if bundle.Total != 0 || len(bundle.Entry) != 0 {
		t.Errorf("got %d observations, want 0", bundle.Total)
	}
}

func TestListObservations_EmptyResult_UnknownPatient(t *testing.T) {
	r, _ := newTestRouter(t)
	post(r, "/api/hl7/messages", readFixture(t, "oru_r01_valid.hl7"))

	rec := get(r, "/fhir/Observation?patient=no-such-patient")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (an unknown patient id is an empty search result, not a 404); body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if bundle := decodeBundle(t, rec.Body.Bytes()); bundle.Total != 0 {
		t.Errorf("got %d observations, want 0", bundle.Total)
	}
}

func TestListObservations_MissingPatientParam(t *testing.T) {
	r, _ := newTestRouter(t)
	rec := get(r, "/fhir/Observation")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
