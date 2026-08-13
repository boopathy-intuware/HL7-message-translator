package mapper

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"hl7-message-translator/backend/hl7"
)

// Fixtures live under ../hl7/testdata and are shared with the hl7 package's
// own parser tests — they are synthetic/hand-written and must never contain
// real patient data.
func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "hl7", "testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %q: %v", name, err)
	}
	return string(data)
}

func parseFixture(t *testing.T, name string) *hl7.Message {
	t.Helper()
	raw := strings.ReplaceAll(readFixture(t, name), "\n", "\r")
	msg, err := hl7.Parse(raw)
	if err != nil {
		t.Fatalf("hl7.Parse(%q): unexpected error: %v", name, err)
	}
	return msg
}

// assertJSONEqual marshals got and compares it against wantJSON structurally
// (via generic interface{} values), so key order and whitespace in wantJSON
// don't matter — only the actual JSON shape and values do.
func assertJSONEqual(t *testing.T, label string, got any, wantJSON string) {
	t.Helper()

	gotBytes, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("%s: json.Marshal(got): %v", label, err)
	}

	var gotVal, wantVal any
	if err := json.Unmarshal(gotBytes, &gotVal); err != nil {
		t.Fatalf("%s: json.Unmarshal(got): %v", label, err)
	}
	if err := json.Unmarshal([]byte(wantJSON), &wantVal); err != nil {
		t.Fatalf("%s: json.Unmarshal(want): %v", label, err)
	}

	if !reflect.DeepEqual(gotVal, wantVal) {
		gotIndented, _ := json.MarshalIndent(gotVal, "", "  ")
		wantIndented, _ := json.MarshalIndent(wantVal, "", "  ")
		t.Errorf("%s JSON mismatch:\ngot:\n%s\nwant:\n%s", label, gotIndented, wantIndented)
	}
}

func TestMapADTA01(t *testing.T) {
	tests := []struct {
		name    string
		msg     *hl7.Message // set directly to bypass hl7.Parse for error-path cases
		fixture string       // used when msg is nil
		wantErr bool

		wantPatientJSON   string
		wantEncounterJSON string
	}{
		{
			name:    "valid ADT^A01 message maps to Patient + Encounter",
			fixture: "adt_a01_valid.hl7",
			wantPatientJSON: `{
				"resourceType": "Patient",
				"id": "123456",
				"identifier": [{"value": "123456"}],
				"name": [{"family": "DOE", "given": "JOHN"}],
				"birthDate": "19800101",
				"gender": "male"
			}`,
			wantEncounterJSON: `{
				"resourceType": "Encounter",
				"id": "MSG00001",
				"status": "in-progress",
				"class": {"code": "I"},
				"subject": {"reference": "Patient/123456"},
				"period": {"start": "20260812093000"}
			}`,
		},
		{
			name: "message missing PID segment errors",
			msg: &hl7.Message{
				MSH: hl7.MSH{MessageControlID: "MSG90001"},
				PV1: &hl7.PV1{PatientClass: "I"},
			},
			wantErr: true,
		},
		{
			name: "message missing PV1 segment errors",
			msg: &hl7.Message{
				MSH: hl7.MSH{MessageControlID: "MSG90002"},
				PID: &hl7.PID{PatientID: "999999"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.msg
			if msg == nil {
				msg = parseFixture(t, tt.fixture)
			}

			patient, encounter, err := MapADTA01(msg)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("MapADTA01() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("MapADTA01() unexpected error: %v", err)
			}

			assertJSONEqual(t, "Patient", patient, tt.wantPatientJSON)
			assertJSONEqual(t, "Encounter", encounter, tt.wantEncounterJSON)

			wantSubject := "Patient/" + patient.ID
			if encounter.Subject.Reference != wantSubject {
				t.Errorf("Encounter.Subject.Reference = %q, want %q", encounter.Subject.Reference, wantSubject)
			}
		})
	}
}

func TestMapORUR01(t *testing.T) {
	tests := []struct {
		name    string
		msg     *hl7.Message // set directly to bypass hl7.Parse for error-path cases
		fixture string       // used when msg is nil
		wantErr bool

		wantPatientJSON string
		wantReportJSON  string
		wantObsJSON     []string
	}{
		{
			name:    "valid ORU^R01 message maps to Patient + DiagnosticReport + Observations",
			fixture: "oru_r01_valid.hl7",
			wantPatientJSON: `{
				"resourceType": "Patient",
				"id": "123457",
				"identifier": [{"value": "123457"}],
				"name": [{"family": "DOE", "given": "JANE"}],
				"birthDate": "19750620",
				"gender": "female"
			}`,
			wantReportJSON: `{
				"resourceType": "DiagnosticReport",
				"id": "MSG00002",
				"status": "final",
				"code": {
					"coding": [{"system": "L", "code": "CBC", "display": "COMPLETE BLOOD COUNT"}],
					"text": "COMPLETE BLOOD COUNT"
				},
				"subject": {"reference": "Patient/123457"},
				"result": [
					{"reference": "Observation/MSG00002-1"},
					{"reference": "Observation/MSG00002-2"},
					{"reference": "Observation/MSG00002-3"}
				]
			}`,
			wantObsJSON: []string{
				// numeric result -> valueQuantity
				`{
					"resourceType": "Observation",
					"id": "MSG00002-1",
					"status": "final",
					"code": {
						"coding": [{"system": "LN", "code": "2345-7", "display": "Glucose"}],
						"text": "Glucose"
					},
					"subject": {"reference": "Patient/123457"},
					"valueQuantity": {"value": 95, "unit": "mg/dL"}
				}`,
				// plain-text result -> valueString
				`{
					"resourceType": "Observation",
					"id": "MSG00002-2",
					"status": "final",
					"code": {
						"coding": [{"system": "LN", "code": "33747-0", "display": "Specimen Comment"}],
						"text": "Specimen Comment"
					},
					"subject": {"reference": "Patient/123457"},
					"valueString": "Sample slightly hemolyzed"
				}`,
				// coded result whose value doesn't parse as numeric -> valueString
				`{
					"resourceType": "Observation",
					"id": "MSG00002-3",
					"status": "final",
					"code": {
						"coding": [{"system": "LN", "code": "48016-2", "display": "COVID Result"}],
						"text": "COVID Result"
					},
					"subject": {"reference": "Patient/123457"},
					"valueString": "NEGATIVE^Negative^L"
				}`,
			},
		},
		{
			name: "message missing PID segment errors",
			msg: &hl7.Message{
				MSH: hl7.MSH{MessageControlID: "MSG90003"},
				OBR: &hl7.OBR{SetID: "1"},
			},
			wantErr: true,
		},
		{
			name: "message missing OBR segment errors",
			msg: &hl7.Message{
				MSH: hl7.MSH{MessageControlID: "MSG90004"},
				PID: &hl7.PID{PatientID: "999999"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.msg
			if msg == nil {
				msg = parseFixture(t, tt.fixture)
			}

			patient, report, observations, err := MapORUR01(msg)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("MapORUR01() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("MapORUR01() unexpected error: %v", err)
			}

			assertJSONEqual(t, "Patient", patient, tt.wantPatientJSON)
			assertJSONEqual(t, "DiagnosticReport", report, tt.wantReportJSON)

			if len(observations) != len(tt.wantObsJSON) {
				t.Fatalf("got %d observations, want %d", len(observations), len(tt.wantObsJSON))
			}
			for i, wantJSON := range tt.wantObsJSON {
				assertJSONEqual(t, fmt.Sprintf("Observation[%d]", i), observations[i], wantJSON)
			}

			wantSubject := "Patient/" + patient.ID
			if report.Subject.Reference != wantSubject {
				t.Errorf("DiagnosticReport.Subject.Reference = %q, want %q", report.Subject.Reference, wantSubject)
			}
			for i, obs := range observations {
				if obs.Subject.Reference != wantSubject {
					t.Errorf("Observation[%d].Subject.Reference = %q, want %q", i, obs.Subject.Reference, wantSubject)
				}
				wantResultRef := "Observation/" + obs.ID
				if report.Result[i].Reference != wantResultRef {
					t.Errorf("DiagnosticReport.Result[%d].Reference = %q, want %q", i, report.Result[i].Reference, wantResultRef)
				}
			}
		})
	}
}
