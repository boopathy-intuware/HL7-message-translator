package ack

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hl7-message-translator/backend/hl7"
)

// Fixtures live under ../hl7/testdata and are shared with the hl7 and
// mapper packages' own tests — they are synthetic/hand-written and must
// never contain real patient data.
func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "hl7", "testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %q: %v", name, err)
	}
	return strings.ReplaceAll(string(data), "\n", "\r")
}

var fixedNow = time.Date(2026, 8, 12, 9, 30, 0, 0, time.UTC)

// parsedACK splits a generated ACK string into its MSH and MSA fields, both
// still including their leading "MSH"/"MSA" segment ID at index 0.
func parsedACK(t *testing.T, ackMsg string) (msh, msa []string) {
	t.Helper()
	segments := strings.Split(strings.Trim(ackMsg, "\r"), "\r")
	if len(segments) != 2 {
		t.Fatalf("Generate() produced %d segments, want 2 (MSH, MSA): %q", len(segments), ackMsg)
	}
	if !strings.HasPrefix(segments[0], "MSH") {
		t.Fatalf("first segment = %q, want MSH", segments[0])
	}
	if !strings.HasPrefix(segments[1], "MSA") {
		t.Fatalf("second segment = %q, want MSA", segments[1])
	}
	return strings.Split(segments[0], "|"), strings.Split(segments[1], "|")
}

func TestGenerate_SuccessfulParse(t *testing.T) {
	raw := readFixture(t, "adt_a01_valid.hl7")
	msg, err := hl7.Parse(raw)
	if err != nil {
		t.Fatalf("hl7.Parse: unexpected error: %v", err)
	}

	got := Generate(raw, msg, nil, nil, fixedNow)
	msh, msa := parsedACK(t, got)

	if msa[1] != "AA" {
		t.Errorf("MSA-1 = %q, want AA", msa[1])
	}
	if msa[2] != "MSG00001" {
		t.Errorf("MSA-2 = %q, want %q (inbound control ID)", msa[2], "MSG00001")
	}
	if len(msa) > 3 {
		t.Errorf("MSA has an error text field on a successful ACK: %v", msa)
	}
	if got, want := msh[5], "SYNTH_HOSPITAL"; got != want { // MSH-6, receiving facility
		t.Errorf("MSH-6 = %q, want %q (inbound sending facility)", got, want)
	}
	if got, want := msh[6], "20260812093000"; got != want { // MSH-7
		t.Errorf("MSH-7 = %q, want %q", got, want)
	}
	if got, want := msh[9], "MSG00001-ACK"; got != want { // MSH-10
		t.Errorf("MSH-10 = %q, want %q", got, want)
	}
}

func TestGenerate_ValidORU(t *testing.T) {
	raw := readFixture(t, "oru_r01_valid.hl7")
	msg, err := hl7.Parse(raw)
	if err != nil {
		t.Fatalf("hl7.Parse: unexpected error: %v", err)
	}

	_, msa := parsedACK(t, Generate(raw, msg, nil, nil, fixedNow))
	if msa[1] != "AA" {
		t.Errorf("MSA-1 = %q, want AA", msa[1])
	}
	if msa[2] != "MSG00002" {
		t.Errorf("MSA-2 = %q, want %q", msa[2], "MSG00002")
	}
}

func TestGenerate_ParseFailure_MissingMSH(t *testing.T) {
	raw := readFixture(t, "missing_msh.hl7")
	msg, parseErr := hl7.Parse(raw)
	if parseErr == nil {
		t.Fatalf("hl7.Parse: expected an error for a message with no MSH segment")
	}
	if msg != nil {
		t.Fatalf("hl7.Parse: expected a nil message alongside the error, got %+v", msg)
	}

	_, msa := parsedACK(t, Generate(raw, msg, parseErr, nil, fixedNow))
	if msa[1] != "AE" {
		t.Errorf("MSA-1 = %q, want AE", msa[1])
	}
	if msa[2] != "" {
		t.Errorf("MSA-2 = %q, want empty (no MSH segment to recover a control ID from)", msa[2])
	}
	if len(msa) < 4 || msa[3] != parseErr.Error() {
		t.Errorf("MSA-3 = %v, want the parse error text %q", msa, parseErr.Error())
	}
}

func TestGenerate_ParseFailure_MSHTruncated(t *testing.T) {
	raw := readFixture(t, "msh_truncated.hl7") // MSH-1 through MSH-5 only, no MSH-10
	msg, parseErr := hl7.Parse(raw)
	if parseErr == nil {
		t.Fatalf("hl7.Parse: expected an error for a truncated MSH segment")
	}

	_, msa := parsedACK(t, Generate(raw, msg, parseErr, nil, fixedNow))
	if msa[1] != "AE" {
		t.Errorf("MSA-1 = %q, want AE", msa[1])
	}
	if msa[2] != "" {
		t.Errorf("MSA-2 = %q, want empty (MSH present but too short to carry MSH-10)", msa[2])
	}
}

func TestGenerate_MappingFailure(t *testing.T) {
	raw := readFixture(t, "adt_a01_valid.hl7")
	msg, err := hl7.Parse(raw)
	if err != nil {
		t.Fatalf("hl7.Parse: unexpected error: %v", err)
	}
	mappingErr := errors.New("mapper: message has no PV1 segment")

	_, msa := parsedACK(t, Generate(raw, msg, nil, mappingErr, fixedNow))
	if msa[1] != "AE" {
		t.Errorf("MSA-1 = %q, want AE (mapping failed even though hl7.Parse succeeded)", msa[1])
	}
	if msa[2] != "MSG00001" {
		t.Errorf("MSA-2 = %q, want %q (control ID still recoverable from the parsed MSH)", msa[2], "MSG00001")
	}
	if len(msa) < 4 || msa[3] != mappingErr.Error() {
		t.Errorf("MSA-3 = %v, want the mapping error text %q", msa, mappingErr.Error())
	}
}
