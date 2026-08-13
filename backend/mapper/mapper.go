// Package mapper converts parsed HL7v2 messages into simplified FHIR R4
// JSON-shaped resources. It does not aim for full spec compliance — only
// the fields needed to round-trip an ADT^A01 (Patient + Encounter) or
// ORU^R01 (Patient + DiagnosticReport + Observation) message.
package mapper

import (
	"fmt"
	"strconv"

	"hl7-message-translator/backend/hl7"
)

// Identifier is a simplified FHIR Identifier.
type Identifier struct {
	Value string `json:"value"`
}

// HumanName is a simplified FHIR HumanName.
type HumanName struct {
	Family string `json:"family"`
	Given  string `json:"given"`
}

// Reference is a simplified FHIR Reference, e.g. {"reference": "Patient/123"}.
type Reference struct {
	Reference string `json:"reference"`
}

// Coding is a simplified FHIR Coding.
type Coding struct {
	System  string `json:"system,omitempty"`
	Code    string `json:"code"`
	Display string `json:"display,omitempty"`
}

// CodeableConcept is a simplified FHIR CodeableConcept.
type CodeableConcept struct {
	Coding []Coding `json:"coding"`
	Text   string   `json:"text,omitempty"`
}

// Period is a simplified FHIR Period.
type Period struct {
	Start string `json:"start,omitempty"`
}

// Quantity is a simplified FHIR Quantity.
type Quantity struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit,omitempty"`
}

// Patient is a simplified FHIR R4 Patient resource, derived from a PID
// segment.
type Patient struct {
	ResourceType string       `json:"resourceType"`
	ID           string       `json:"id"`
	Identifier   []Identifier `json:"identifier"`
	Name         []HumanName  `json:"name"`
	BirthDate    string       `json:"birthDate,omitempty"`
	Gender       string       `json:"gender"`
}

// Encounter is a simplified FHIR R4 Encounter resource, derived from a PV1
// segment.
type Encounter struct {
	ResourceType string    `json:"resourceType"`
	ID           string    `json:"id"`
	Status       string    `json:"status"`
	Class        Coding    `json:"class"`
	Subject      Reference `json:"subject"`
	Period       Period    `json:"period"`
}

// DiagnosticReport is a simplified FHIR R4 DiagnosticReport resource,
// derived from an OBR segment plus the Observations built from its OBX
// segments.
type DiagnosticReport struct {
	ResourceType string          `json:"resourceType"`
	ID           string          `json:"id"`
	Status       string          `json:"status"`
	Code         CodeableConcept `json:"code"`
	Subject      Reference       `json:"subject"`
	Result       []Reference     `json:"result"`
}

// Observation is a simplified FHIR R4 Observation resource, derived from a
// single OBX segment. Exactly one of ValueQuantity or ValueString is set,
// mirroring FHIR's polymorphic value[x].
type Observation struct {
	ResourceType  string          `json:"resourceType"`
	ID            string          `json:"id"`
	Status        string          `json:"status"`
	Code          CodeableConcept `json:"code"`
	Subject       Reference       `json:"subject"`
	ValueQuantity *Quantity       `json:"valueQuantity,omitempty"`
	ValueString   string          `json:"valueString,omitempty"`
}

// MapADTA01 converts an ADT^A01 message's PID and PV1 segments into a
// simplified FHIR Patient and Encounter.
func MapADTA01(msg *hl7.Message) (*Patient, *Encounter, error) {
	if msg.PID == nil {
		return nil, nil, fmt.Errorf("mapper: message has no PID segment")
	}
	if msg.PV1 == nil {
		return nil, nil, fmt.Errorf("mapper: message has no PV1 segment")
	}

	patient := mapPatient(msg.PID)
	encounter := &Encounter{
		ResourceType: "Encounter",
		ID:           msg.MSH.MessageControlID,
		Status:       "in-progress", // ADT^A01 announces an admit: the encounter is under way, not finished.
		Class:        Coding{Code: msg.PV1.PatientClass},
		Subject:      Reference{Reference: "Patient/" + patient.ID},
		Period:       Period{Start: msg.PV1.AdmitDateTime},
	}

	return &patient, encounter, nil
}

// MapORUR01 converts an ORU^R01 message's PID, OBR, and OBX segments into a
// simplified FHIR Patient, DiagnosticReport, and one Observation per OBX.
// The DiagnosticReport's Result references each Observation by ID, and
// every Observation's Subject references the Patient by ID.
func MapORUR01(msg *hl7.Message) (*Patient, *DiagnosticReport, []Observation, error) {
	if msg.PID == nil {
		return nil, nil, nil, fmt.Errorf("mapper: message has no PID segment")
	}
	if msg.OBR == nil {
		return nil, nil, nil, fmt.Errorf("mapper: message has no OBR segment")
	}

	patient := mapPatient(msg.PID)
	subject := Reference{Reference: "Patient/" + patient.ID}

	observations := make([]Observation, 0, len(msg.OBX))
	results := make([]Reference, 0, len(msg.OBX))
	for _, obx := range msg.OBX {
		obs := mapObservation(msg.MSH.MessageControlID, obx, subject)
		observations = append(observations, obs)
		results = append(results, Reference{Reference: "Observation/" + obs.ID})
	}

	report := &DiagnosticReport{
		ResourceType: "DiagnosticReport",
		ID:           msg.MSH.MessageControlID,
		Status:       "final",
		Code:         mapCodeableConcept(msg.OBR.UniversalServiceID),
		Subject:      subject,
		Result:       results,
	}

	return &patient, report, observations, nil
}

func mapPatient(pid *hl7.PID) Patient {
	return Patient{
		ResourceType: "Patient",
		ID:           pid.PatientID,
		Identifier:   []Identifier{{Value: pid.PatientID}},
		Name:         []HumanName{{Family: pid.Name.Family, Given: pid.Name.Given}},
		BirthDate:    pid.DOB,
		Gender:       mapGender(pid.Sex),
	}
}

func mapGender(sex string) string {
	switch sex {
	case "M":
		return "male"
	case "F":
		return "female"
	default:
		return "unknown"
	}
}

func mapCodeableConcept(ce hl7.CodedElement) CodeableConcept {
	return CodeableConcept{
		Coding: []Coding{{System: ce.CodingSystem, Code: ce.Code, Display: ce.Text}},
		Text:   ce.Text,
	}
}

// mapObservation builds an Observation for a single OBX result. Its ID is
// derived from the owning message's control ID plus the OBX set ID, since
// a later read API needs a stable, unique way to look up an Observation
// (and for DiagnosticReport.result to reference it) without HL7 providing
// one directly.
func mapObservation(messageControlID string, obx hl7.OBX, subject Reference) Observation {
	obs := Observation{
		ResourceType: "Observation",
		ID:           messageControlID + "-" + obx.SetID,
		Status:       "final",
		Code:         mapCodeableConcept(obx.ObservationID),
		Subject:      subject,
	}

	if value, err := strconv.ParseFloat(obx.Value, 64); err == nil {
		obs.ValueQuantity = &Quantity{Value: value, Unit: obx.Units}
	} else {
		obs.ValueString = obx.Value
	}

	return obs
}
