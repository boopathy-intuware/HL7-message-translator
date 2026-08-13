// Package hl7 implements a minimal parser for HL7v2 pipe-delimited messages.
package hl7

// Message is the parsed representation of a single HL7v2 message.
// PID, PV1, and OBR are nil when the message contains no such segment. OBX
// is nil when the message contains no OBX segment, and holds one entry per
// OBX segment otherwise (a message may report several observations).
type Message struct {
	MSH MSH
	PID *PID
	PV1 *PV1
	OBR *OBR
	OBX []OBX
}

// MSH holds the fields of the HL7v2 MSH (Message Header) segment that this
// parser understands. Field numbering in HL7v2 starts at 1, where MSH-1 is
// the field separator character itself.
type MSH struct {
	FieldSeparator     string // MSH-1
	EncodingCharacters string // MSH-2, e.g. "^~\&"
	SendingFacility    string // MSH-4
	Timestamp          string // MSH-7
	MessageType        string // MSH-9 component 1, e.g. "ADT"
	TriggerEvent       string // MSH-9 component 2, e.g. "A01"
	MessageControlID   string // MSH-10
}

// PID holds the fields of the HL7v2 PID (Patient Identification) segment
// that this parser understands.
type PID struct {
	PatientID string // PID-3, first component
	Name      PatientName
	DOB       string // PID-7
	Sex       string // PID-8
}

// PatientName is the parsed form of PID-5.
type PatientName struct {
	Family string
	Given  string
	Middle string
	Suffix string
}

// PV1 holds the fields of the HL7v2 PV1 (Patient Visit) segment that this
// parser understands. PV1-44 (Admit Date/Time) is far to the right of the
// segment because HL7v2 field numbering is positional — a message that
// carries it must still spell out every unused field before it.
type PV1 struct {
	PatientClass     string // PV1-2
	AssignedLocation string // PV1-3, e.g. "ER^101^1^SYNTH_HOSPITAL" (point of care^room^bed^facility)
	AdmitDateTime    string // PV1-44
}

// OBR holds the fields of the HL7v2 OBR (Observation Request) segment that
// this parser understands. A message reports at most one OBR (the order
// the following OBX results belong to).
type OBR struct {
	SetID              string       // OBR-1
	UniversalServiceID CodedElement // OBR-4
}

// OBX holds the fields of the HL7v2 OBX (Observation/Result) segment that
// this parser understands. A message carries one OBX per reported result,
// so Message.OBX is a slice.
type OBX struct {
	SetID         string       // OBX-1
	ValueType     string       // OBX-2, e.g. "NM" (numeric) or "ST" (string)
	ObservationID CodedElement // OBX-3
	Value         string       // OBX-5
	Units         string       // OBX-6
}

// CodedElement is the parsed form of an HL7v2 CE (coded element) field:
// identifier^text^coding system (e.g. "2345-7^Glucose^LN").
type CodedElement struct {
	Code         string
	Text         string
	CodingSystem string
}
