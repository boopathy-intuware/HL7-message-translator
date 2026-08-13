package hl7

import (
	"fmt"
	"strings"
)

const (
	fieldSep     = "|"
	componentSep = "^"
)

// ParseError describes a failure to parse an HL7v2 message or one of its
// segments. Segment is empty when the error applies to the message as a
// whole rather than to one segment.
type ParseError struct {
	Segment string
	Reason  string
}

func (e *ParseError) Error() string {
	if e.Segment == "" {
		return fmt.Sprintf("hl7: %s", e.Reason)
	}
	return fmt.Sprintf("hl7: %s segment: %s", e.Segment, e.Reason)
}

// Parse parses a raw HL7v2 message into a Message. Segments are split on
// \r (the HL7v2 standard segment terminator); bare \n is also accepted
// since hand-typed test messages often lack proper HL7 line endings.
//
// Parse returns an error if the message has no segments, if it is missing
// the required MSH segment, or if the MSH or PID segments do not carry
// enough fields to populate the fields this package understands.
func Parse(raw string) (*Message, error) {
	segments := splitSegments(raw)
	if len(segments) == 0 {
		return nil, &ParseError{Reason: "message has no segments"}
	}

	msg := &Message{}
	sawMSH := false

	for _, segment := range segments {
		switch segmentID(segment) {
		case "MSH":
			msh, err := parseMSH(segment)
			if err != nil {
				return nil, err
			}
			msg.MSH = *msh
			sawMSH = true
		case "PID":
			pid, err := parsePID(segment)
			if err != nil {
				return nil, err
			}
			msg.PID = pid
		case "PV1":
			pv1, err := parsePV1(segment)
			if err != nil {
				return nil, err
			}
			msg.PV1 = pv1
		case "OBR":
			obr, err := parseOBR(segment)
			if err != nil {
				return nil, err
			}
			msg.OBR = obr
		case "OBX":
			obx, err := parseOBX(segment)
			if err != nil {
				return nil, err
			}
			msg.OBX = append(msg.OBX, *obx)
		}
	}

	if !sawMSH {
		return nil, &ParseError{Segment: "MSH", Reason: "missing required MSH segment"}
	}

	return msg, nil
}

// splitSegments splits a raw message into segment strings on \r, \n, or
// \r\n, discarding any blank segments produced by trailing terminators.
func splitSegments(raw string) []string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	var segments []string
	for part := range strings.SplitSeq(normalized, "\n") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		segments = append(segments, part)
	}
	return segments
}

// splitFields splits a segment into its pipe-delimited fields.
func splitFields(segment string) []string {
	return strings.Split(segment, fieldSep)
}

// splitComponents splits a field into its caret-delimited components.
func splitComponents(field string) []string {
	if field == "" {
		return nil
	}
	return strings.Split(field, componentSep)
}

// field returns the fields[index] (0-indexed), or "" if index is out of range.
func field(fields []string, index int) string {
	if index < 0 || index >= len(fields) {
		return ""
	}
	return fields[index]
}

// component returns the comp-th (0-indexed) component of fields[index]
// (0-indexed), or "" if either index is out of range.
func component(fields []string, index, comp int) string {
	comps := splitComponents(field(fields, index))
	if comp < 0 || comp >= len(comps) {
		return ""
	}
	return comps[comp]
}

// segmentID returns the leading segment identifier (e.g. "MSH", "PID") for
// a raw segment string, without splitting the whole segment on fieldSep.
func segmentID(segment string) string {
	id, _, _ := strings.Cut(segment, fieldSep)
	return id
}

// parseMSH parses an MSH segment. Unlike every other segment type, the
// character immediately after "MSH" is the field separator itself (usually
// "|"), so splitting the segment on fieldSep does not recover MSH-1 — the
// split's first element is always the literal "MSH", and the
// encoding-characters field (MSH-2, e.g. "^~\&") lands in the first split
// position instead of field 1. Every subsequent MSH field is therefore
// shifted by one relative to how fields are indexed in other segments
// (e.g. PID): MSH-N lives at split index N-1, not split index N.
func parseMSH(segment string) (*MSH, error) {
	if len(segment) < 4 || segment[:3] != "MSH" {
		return nil, &ParseError{Segment: "MSH", Reason: "segment does not start with MSH plus a field separator"}
	}
	fieldSeparator := string(segment[3])

	fields := splitFields(segment)

	// MSH-10 (message control ID) is the last field this parser requires,
	// which lands at split index 9.
	const minFields = 10
	if len(fields) < minFields {
		return nil, &ParseError{Segment: "MSH", Reason: "missing required fields (need at least MSH-1 through MSH-10)"}
	}

	return &MSH{
		FieldSeparator:     fieldSeparator,
		EncodingCharacters: field(fields, 1), // MSH-2
		SendingFacility:    field(fields, 3), // MSH-4
		Timestamp:          field(fields, 6), // MSH-7
		MessageType:        component(fields, 8, 0), // MSH-9.1
		TriggerEvent:       component(fields, 8, 1), // MSH-9.2
		MessageControlID:   field(fields, 9),        // MSH-10
	}, nil
}

// parsePID parses a PID segment. PID fields follow the generic HL7v2
// indexing convention: PID-N lives at split index N (split index 0 is the
// literal segment ID "PID").
func parsePID(segment string) (*PID, error) {
	fields := splitFields(segment)

	// PID-8 (sex) is the last field this parser requires, at split index 8.
	const minFields = 9
	if len(fields) < minFields {
		return nil, &ParseError{Segment: "PID", Reason: "missing required fields (need at least PID-1 through PID-8)"}
	}

	return &PID{
		PatientID: component(fields, 3, 0), // PID-3.1
		Name: PatientName{
			Family: component(fields, 5, 0), // PID-5.1
			Given:  component(fields, 5, 1), // PID-5.2
			Middle: component(fields, 5, 2), // PID-5.3
			Suffix: component(fields, 5, 3), // PID-5.4
		},
		DOB: field(fields, 7), // PID-7
		Sex: field(fields, 8), // PID-8
	}, nil
}

// parsePV1 parses a PV1 segment. PV1 fields follow the generic HL7v2
// indexing convention: PV1-N lives at split index N. Only patient class
// (PV1-2) is required; assigned location and admit date/time are read
// through field(), which tolerates a segment truncated before it reaches
// them.
func parsePV1(segment string) (*PV1, error) {
	fields := splitFields(segment)

	// PV1-2 (patient class) is the last field this parser requires, at split index 2.
	const minFields = 3
	if len(fields) < minFields {
		return nil, &ParseError{Segment: "PV1", Reason: "missing required fields (need at least PV1-1 through PV1-2)"}
	}

	return &PV1{
		PatientClass:     field(fields, 2),  // PV1-2
		AssignedLocation: field(fields, 3),  // PV1-3
		AdmitDateTime:    field(fields, 44), // PV1-44
	}, nil
}

// parseOBR parses an OBR segment. OBR fields follow the generic HL7v2
// indexing convention: OBR-N lives at split index N.
func parseOBR(segment string) (*OBR, error) {
	fields := splitFields(segment)

	// OBR-4 (universal service ID) is the last field this parser requires, at split index 4.
	const minFields = 5
	if len(fields) < minFields {
		return nil, &ParseError{Segment: "OBR", Reason: "missing required fields (need at least OBR-1 through OBR-4)"}
	}

	return &OBR{
		SetID: field(fields, 1), // OBR-1
		UniversalServiceID: CodedElement{
			Code:         component(fields, 4, 0), // OBR-4.1
			Text:         component(fields, 4, 1), // OBR-4.2
			CodingSystem: component(fields, 4, 2), // OBR-4.3
		},
	}, nil
}

// parseOBX parses an OBX segment. OBX fields follow the generic HL7v2
// indexing convention: OBX-N lives at split index N.
func parseOBX(segment string) (*OBX, error) {
	fields := splitFields(segment)

	// OBX-5 (observation value) is the last field this parser requires, at split index 5.
	const minFields = 6
	if len(fields) < minFields {
		return nil, &ParseError{Segment: "OBX", Reason: "missing required fields (need at least OBX-1 through OBX-5)"}
	}

	return &OBX{
		SetID:     field(fields, 1), // OBX-1
		ValueType: field(fields, 2), // OBX-2
		ObservationID: CodedElement{
			Code:         component(fields, 3, 0), // OBX-3.1
			Text:         component(fields, 3, 1), // OBX-3.2
			CodingSystem: component(fields, 3, 2), // OBX-3.3
		},
		Value: field(fields, 5), // OBX-5
		Units: field(fields, 6), // OBX-6
	}, nil
}
