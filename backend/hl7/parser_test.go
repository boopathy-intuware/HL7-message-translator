package hl7

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Fixtures under testdata/ are synthetic and hand-written for testing —
// they must never contain real patient data. Each fixture is stored with
// LF line endings (safe to author, diff, and check into git); tests derive
// the CR-terminated form in-code where the standard HL7v2 terminator
// matters, since a bare \r in a checked-in file is easy to mangle.
func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %q: %v", name, err)
	}
	return string(data)
}

func TestParse(t *testing.T) {
	adtLF := readFixture(t, "adt_a01_valid.hl7")
	adtCR := strings.ReplaceAll(adtLF, "\n", "\r")

	tests := []struct {
		name string
		raw  string

		// For cases that should parse successfully.
		wantMSH MSH
		wantPID *PID
		wantPV1 *PV1
		wantOBR *OBR
		wantOBX []OBX

		// For cases that should fail: wantErrSegment is required, and must
		// match ParseError.Segment. wantErrReason, if non-empty, must be a
		// substring of ParseError.Reason.
		wantErrSegment string
		wantErrReason  string
	}{
		{
			name: "valid ADT^A01 message, CR terminated",
			raw:  adtCR,
			wantMSH: MSH{
				FieldSeparator:     "|",
				EncodingCharacters: `^~\&`,
				SendingFacility:    "SYNTH_HOSPITAL",
				Timestamp:          "20260812093000",
				MessageType:        "ADT",
				TriggerEvent:       "A01",
				MessageControlID:   "MSG00001",
			},
			wantPID: &PID{
				PatientID: "123456",
				Name:      PatientName{Family: "DOE", Given: "JOHN"},
				DOB:       "19800101",
				Sex:       "M",
			},
			wantPV1: &PV1{
				PatientClass:     "I",
				AssignedLocation: "ER^101^1^SYNTH_HOSPITAL",
				AdmitDateTime:    "20260812093000",
			},
		},
		{
			name: "hand-typed message with bare newlines parses identically",
			raw:  adtLF,
			wantMSH: MSH{
				FieldSeparator:     "|",
				EncodingCharacters: `^~\&`,
				SendingFacility:    "SYNTH_HOSPITAL",
				Timestamp:          "20260812093000",
				MessageType:        "ADT",
				TriggerEvent:       "A01",
				MessageControlID:   "MSG00001",
			},
			wantPID: &PID{
				PatientID: "123456",
				Name:      PatientName{Family: "DOE", Given: "JOHN"},
				DOB:       "19800101",
				Sex:       "M",
			},
			wantPV1: &PV1{
				PatientClass:     "I",
				AssignedLocation: "ER^101^1^SYNTH_HOSPITAL",
				AdmitDateTime:    "20260812093000",
			},
		},
		{
			name: "valid ORU^R01 message with OBR and repeated OBX",
			raw:  strings.ReplaceAll(readFixture(t, "oru_r01_valid.hl7"), "\n", "\r"),
			wantMSH: MSH{
				FieldSeparator:     "|",
				EncodingCharacters: `^~\&`,
				SendingFacility:    "SYNTH_LAB",
				Timestamp:          "20260812101500",
				MessageType:        "ORU",
				TriggerEvent:       "R01",
				MessageControlID:   "MSG00002",
			},
			wantPID: &PID{
				PatientID: "123457",
				Name:      PatientName{Family: "DOE", Given: "JANE"},
				DOB:       "19750620",
				Sex:       "F",
			},
			wantOBR: &OBR{
				SetID: "1",
				UniversalServiceID: CodedElement{
					Code:         "CBC",
					Text:         "COMPLETE BLOOD COUNT",
					CodingSystem: "L",
				},
			},
			wantOBX: []OBX{
				{
					SetID:         "1",
					ValueType:     "NM",
					ObservationID: CodedElement{Code: "2345-7", Text: "Glucose", CodingSystem: "LN"},
					Value:         "95",
					Units:         "mg/dL",
				},
				{
					SetID:         "2",
					ValueType:     "ST",
					ObservationID: CodedElement{Code: "33747-0", Text: "Specimen Comment", CodingSystem: "LN"},
					Value:         "Sample slightly hemolyzed",
				},
				{
					SetID:         "3",
					ValueType:     "CE",
					ObservationID: CodedElement{Code: "48016-2", Text: "COVID Result", CodingSystem: "LN"},
					Value:         "NEGATIVE^Negative^L",
				},
			},
		},
		{
			name: "MSH-9 with no trigger event component does not panic or error",
			raw:  strings.ReplaceAll(readFixture(t, "msh_no_trigger_event.hl7"), "\n", "\r"),
			wantMSH: MSH{
				FieldSeparator:     "|",
				EncodingCharacters: `^~\&`,
				SendingFacility:    "SYNTH_HOSPITAL",
				Timestamp:          "20260812093000",
				MessageType:        "ADT",
				TriggerEvent:       "", // no "^A01" suffix present
				MessageControlID:   "MSG00003",
			},
			wantPID: nil,
		},
		{
			name:           "message missing MSH segment entirely",
			raw:            strings.ReplaceAll(readFixture(t, "missing_msh.hl7"), "\n", "\r"),
			wantErrSegment: "MSH",
		},
		{
			name:           "MSH segment truncated below minimum required fields",
			raw:            strings.ReplaceAll(readFixture(t, "msh_truncated.hl7"), "\n", "\r"), // MSH-1 through MSH-5 only
			wantErrSegment: "MSH",
			wantErrReason:  "missing required fields",
		},
		{
			name:           "PID segment truncated below minimum required fields",
			raw:            strings.ReplaceAll(readFixture(t, "pid_truncated.hl7"), "\n", "\r"), // missing PID-8 (sex)
			wantErrSegment: "PID",
		},
		{
			name:           "PV1 segment truncated below minimum required fields",
			raw:            strings.ReplaceAll(readFixture(t, "pv1_truncated.hl7"), "\n", "\r"), // missing PV1-2 (patient class)
			wantErrSegment: "PV1",
			wantErrReason:  "missing required fields",
		},
		{
			name:           "OBR segment truncated below minimum required fields",
			raw:            strings.ReplaceAll(readFixture(t, "obr_truncated.hl7"), "\n", "\r"), // missing OBR-4 (universal service ID)
			wantErrSegment: "OBR",
			wantErrReason:  "missing required fields",
		},
		{
			name:           "OBX segment truncated below minimum required fields",
			raw:            strings.ReplaceAll(readFixture(t, "obx_truncated.hl7"), "\n", "\r"), // missing OBX-5 (observation value)
			wantErrSegment: "OBX",
			wantErrReason:  "missing required fields",
		},
		{
			name: "segment with no field separator is safely ignored",
			raw:  strings.ReplaceAll(readFixture(t, "segment_no_field_separator.hl7"), "\n", "\r"),
			wantMSH: MSH{
				FieldSeparator:     "|",
				EncodingCharacters: `^~\&`,
				SendingFacility:    "SYNTH_HOSPITAL",
				Timestamp:          "20260812093000",
				MessageType:        "ADT",
				TriggerEvent:       "A01",
				MessageControlID:   "MSG00008",
			},
			wantPID: &PID{
				PatientID: "123460",
				Name:      PatientName{Family: "GARCIA", Given: "LEE"},
				DOB:       "19881111",
				Sex:       "F",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := Parse(tt.raw)

			if tt.wantErrSegment != "" {
				if err == nil {
					t.Fatalf("Parse() = %+v, nil; want error with Segment %q", msg, tt.wantErrSegment)
				}
				var parseErr *ParseError
				if !errors.As(err, &parseErr) {
					t.Fatalf("Parse() error type = %T, want *ParseError", err)
				}
				if parseErr.Segment != tt.wantErrSegment {
					t.Errorf("ParseError.Segment = %q, want %q", parseErr.Segment, tt.wantErrSegment)
				}
				if tt.wantErrReason != "" && !strings.Contains(parseErr.Reason, tt.wantErrReason) {
					t.Errorf("ParseError.Reason = %q, want substring %q", parseErr.Reason, tt.wantErrReason)
				}
				return
			}

			if err != nil {
				t.Fatalf("Parse() unexpected error: %v", err)
			}
			if msg.MSH != tt.wantMSH {
				t.Errorf("MSH = %+v, want %+v", msg.MSH, tt.wantMSH)
			}
			if !equalPID(msg.PID, tt.wantPID) {
				t.Errorf("PID = %+v, want %+v", msg.PID, tt.wantPID)
			}
			if !equalPV1(msg.PV1, tt.wantPV1) {
				t.Errorf("PV1 = %+v, want %+v", msg.PV1, tt.wantPV1)
			}
			if !equalOBR(msg.OBR, tt.wantOBR) {
				t.Errorf("OBR = %+v, want %+v", msg.OBR, tt.wantOBR)
			}
			// reflect.DeepEqual treats a nil slice and []OBX{} as unequal. Test
			// cases with no OBX segments leave tt.wantOBX at its zero value
			// (nil), which matches msg.OBX only because Parse never appends to
			// it when the message carries no OBX segment (see parse.go) — it's
			// not a coincidence, but worth keeping true if that changes.
			if !reflect.DeepEqual(msg.OBX, tt.wantOBX) {
				t.Errorf("OBX = %+v, want %+v", msg.OBX, tt.wantOBX)
			}
		})
	}
}

func equalPID(a, b *PID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func equalPV1(a, b *PV1) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func equalOBR(a, b *OBR) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
