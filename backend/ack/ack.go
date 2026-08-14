// Package ack builds HL7v2 ACK (acknowledgment) messages — an MSH+MSA
// segment pair — in response to an inbound message the server ingested,
// whether or not that message parsed successfully.
package ack

import (
	"strings"
	"time"

	"hl7-message-translator/backend/hl7"
)

// Code is an HL7v2 MSA-1 acknowledgment code.
type Code string

const (
	CodeAccept Code = "AA" // Application Accept
	CodeError  Code = "AE" // Application Error
)

// ourApplication and ourFacility identify this server as the ACK's sender
// (MSH-3, MSH-4). They happen to share a value because this server has no
// separate facility identity from its application identity, not because
// the two MSH fields are interchangeable.
const (
	ourApplication = "HL7-MESSAGE-TRANSLATOR" // MSH-3: Sending Application
	ourFacility    = "HL7-MESSAGE-TRANSLATOR" // MSH-4: Sending Facility
)

// Generate builds an ACK message for an inbound raw HL7v2 message.
//
// msg and parseErr are the result of hl7.Parse(raw): when parseErr is nil,
// the ACK reports AA and echoes msg.MSH.MessageControlID, and the inbound
// message's sending facility (MSH-4) becomes the ACK's receiving facility
// (MSH-6), per the HL7v2 convention of swapping sender/receiver on
// acknowledgment. When parseErr is non-nil, msg is nil (per hl7.Parse's
// contract) so the message control ID is instead recovered via a
// best-effort scan of raw — this succeeds whenever the inbound MSH segment
// itself was well-formed even if some other segment caused the parse
// failure, and falls back to an empty control ID (and thus an
// unmatched-but-still-valid ACK) only when even MSH is unusable.
// mappingErr, if non-nil, is also reported as AE — it represents a mapping
// failure (e.g. an unsupported message type) discovered after hl7.Parse
// already succeeded structurally.
func Generate(raw string, msg *hl7.Message, parseErr error, mappingErr error, now time.Time) string {
	controlID := extractControlID(raw)
	receivingFacility := ""
	if msg != nil {
		receivingFacility = msg.MSH.SendingFacility
		if msg.MSH.MessageControlID != "" {
			controlID = msg.MSH.MessageControlID
		}
	}

	code := CodeAccept
	errText := ""
	switch {
	case parseErr != nil:
		code = CodeError
		errText = parseErr.Error()
	case mappingErr != nil:
		code = CodeError
		errText = mappingErr.Error()
	}

	ackControlID := controlID
	if ackControlID == "" {
		ackControlID = "UNKNOWN"
	}

	msh := strings.Join([]string{
		"MSH", `^~\&`,
		ourApplication, ourFacility, // MSH-3, MSH-4
		"", receivingFacility, // MSH-5 (receiving application, unknown), MSH-6
		now.UTC().Format("20060102150405"), // MSH-7
		"",                                 // MSH-8 (security)
		"ACK",                              // MSH-9
		ackControlID + "-ACK",              // MSH-10 (the ACK's own control ID)
		"P",                                // MSH-11
		"2.3",                              // MSH-12
	}, "|")

	msaFields := []string{"MSA", string(code), controlID}
	if errText != "" {
		msaFields = append(msaFields, errText)
	}
	msa := strings.Join(msaFields, "|")

	return msh + "\r" + msa + "\r"
}

// extractControlID recovers MSH-10 from raw via a lenient, standalone scan
// of the first segment, independent of hl7.Parse — so a control ID can
// still be echoed back even when hl7.Parse rejected the message outright
// (e.g. because a later segment was malformed) and returned no *hl7.Message
// at all.
func extractControlID(raw string) string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	firstSegment, _, _ := strings.Cut(normalized, "\n")

	if len(firstSegment) < 4 || firstSegment[:3] != "MSH" {
		return ""
	}
	fieldSeparator := string(firstSegment[3])

	fields := strings.Split(firstSegment, fieldSeparator)
	// MSH-10 lands at split index 9, the same off-by-one explained in
	// hl7.parseMSH: the character after "MSH" is the separator itself, so
	// MSH-2 (not MSH-1) occupies split index 1.
	const controlIDIndex = 9
	if len(fields) <= controlIDIndex {
		return ""
	}
	return fields[controlIDIndex]
}
