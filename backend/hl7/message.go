// Package hl7 implements a minimal parser for HL7v2 pipe-delimited messages.
package hl7

// Message is the parsed representation of a single HL7v2 message.
// PID is nil when the message contains no PID segment.
type Message struct {
	MSH MSH
	PID *PID
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
