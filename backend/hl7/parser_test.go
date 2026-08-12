package hl7

import (
	"errors"
	"os"
	"path/filepath"
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
		},
		{
			name: "valid ORU^R01 message (MSH + PID only, no OBR/OBX yet)",
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
		})
	}
}

func equalPID(a, b *PID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
