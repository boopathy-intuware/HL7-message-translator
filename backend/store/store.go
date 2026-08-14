// Package store defines the persistence interface for ingested HL7v2
// messages and the FHIR resources derived from them, matching the
// messages/fhir_resources schema in migrations/000001_init_schema.up.sql.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrNotFound is returned by GetMessage and GetPatientByID when no
// matching row exists.
var ErrNotFound = errors.New("store: not found")

const (
	ParseStatusSuccess = "success"
	ParseStatusFailed  = "failed"
)

// NewMessage is the data needed to insert a row into messages.
type NewMessage struct {
	RawMessage  string
	MessageType string
	ParseStatus string
	ErrorDetail *string // nil when ParseStatus is ParseStatusSuccess
}

// MessageSummary is a messages row without its raw_message body, for list
// views where the full raw text isn't needed.
type MessageSummary struct {
	ID          int64     `json:"id"`
	MessageType string    `json:"message_type"`
	ReceivedAt  time.Time `json:"received_at"`
	ParseStatus string    `json:"parse_status"`
	ErrorDetail *string   `json:"error_detail,omitempty"`
}

// Message is a full messages row, including its raw inbound text.
type Message struct {
	ID          int64     `json:"id"`
	RawMessage  string    `json:"raw_message"`
	MessageType string    `json:"message_type"`
	ReceivedAt  time.Time `json:"received_at"`
	ParseStatus string    `json:"parse_status"`
	ErrorDetail *string   `json:"error_detail,omitempty"`
}

// NewFHIRResource is the data needed to insert a row into fhir_resources,
// as part of IngestMessage — message_id isn't included since it's assigned
// by IngestMessage itself once the owning message row is inserted.
type NewFHIRResource struct {
	ResourceType string
	ResourceJSON json.RawMessage
}

// FHIRResource is a fhir_resources row.
type FHIRResource struct {
	ID           int64           `json:"id"`
	MessageID    int64           `json:"message_id"`
	ResourceType string          `json:"resource_type"`
	ResourceJSON json.RawMessage `json:"resource_json"`
	CreatedAt    time.Time       `json:"created_at"`
}

// Store is the persistence interface used by the HTTP handlers. It is
// implemented by *PostgresStore for production use, and can be faked in
// tests without a real database.
type Store interface {
	// IngestMessage atomically records an ingested raw message and every
	// FHIR resource derived from it (resources is empty when the message
	// failed to parse or map), returning the new message's generated ID.
	// Atomicity matters here: a message row marked ParseStatusSuccess
	// must never end up with only some of its resources persisted.
	IngestMessage(ctx context.Context, m NewMessage, resources []NewFHIRResource) (id int64, err error)

	// ListMessages returns every ingested message, most recently received
	// first.
	ListMessages(ctx context.Context) ([]MessageSummary, error)

	// GetMessage returns a message and its derived FHIR resources, or
	// ErrNotFound if no message exists with the given ID.
	GetMessage(ctx context.Context, id int64) (*Message, []FHIRResource, error)

	// Ping reports whether the underlying database is reachable, for the
	// /ready endpoint.
	Ping(ctx context.Context) error

	// GetPatientByID returns the most recently derived Patient resource
	// whose FHIR id matches id, or ErrNotFound if none exists. A patient
	// can be re-derived across several ingested messages (e.g. an ADT
	// admit followed by later lab results); the most recent one wins.
	GetPatientByID(ctx context.Context, id string) (*FHIRResource, error)

	// SearchPatientsByFamilyName returns every Patient resource whose
	// name list contains a family name matching family (case-insensitive
	// substring match), most recently derived first. It returns an empty
	// slice, not an error, when nothing matches.
	SearchPatientsByFamilyName(ctx context.Context, family string) ([]FHIRResource, error)

	// ListObservationsForPatient returns every Observation resource
	// derived from a message that also derived a Patient resource with
	// the given FHIR id — i.e. linked via their shared source message,
	// not via the Observation's own embedded subject reference. It
	// returns an empty slice, not an error, when the patient has no
	// observations (or doesn't exist at all).
	ListObservationsForPatient(ctx context.Context, patientID string) ([]FHIRResource, error)
}
