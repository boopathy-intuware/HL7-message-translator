package store

import (
	"context"
	"database/sql"
	"errors"

	_ "github.com/lib/pq" // registers the "postgres" database/sql driver
)

// PostgresStore is a Store backed by Postgres, matching the schema created
// by migrations/000001_init_schema.up.sql.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore opens a Postgres connection pool for databaseURL (a
// postgres:// connection string, e.g. from the DATABASE_URL env var).
func NewPostgresStore(databaseURL string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}
	return &PostgresStore{db: db}, nil
}

// Close closes the underlying connection pool.
func (s *PostgresStore) Close() error {
	return s.db.Close()
}

func (s *PostgresStore) IngestMessage(ctx context.Context, m NewMessage, resources []NewFHIRResource) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() // no-op once Commit has succeeded

	const insertMessageQ = `
		INSERT INTO messages (raw_message, message_type, parse_status, error_detail)
		VALUES ($1, $2, $3, $4)
		RETURNING id`

	var id int64
	if err := tx.QueryRowContext(ctx, insertMessageQ, m.RawMessage, m.MessageType, m.ParseStatus, m.ErrorDetail).Scan(&id); err != nil {
		return 0, err
	}

	const insertResourceQ = `
		INSERT INTO fhir_resources (message_id, resource_type, resource_json)
		VALUES ($1, $2, $3)`

	for _, r := range resources {
		if _, err := tx.ExecContext(ctx, insertResourceQ, id, r.ResourceType, []byte(r.ResourceJSON)); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *PostgresStore) ListMessages(ctx context.Context) ([]MessageSummary, error) {
	const q = `
		SELECT id, message_type, received_at, parse_status, error_detail
		FROM messages
		ORDER BY received_at DESC`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []MessageSummary
	for rows.Next() {
		var m MessageSummary
		if err := rows.Scan(&m.ID, &m.MessageType, &m.ReceivedAt, &m.ParseStatus, &m.ErrorDetail); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func (s *PostgresStore) GetMessage(ctx context.Context, id int64) (*Message, []FHIRResource, error) {
	const messageQ = `
		SELECT id, raw_message, message_type, received_at, parse_status, error_detail
		FROM messages
		WHERE id = $1`

	var m Message
	err := s.db.QueryRowContext(ctx, messageQ, id).Scan(
		&m.ID, &m.RawMessage, &m.MessageType, &m.ReceivedAt, &m.ParseStatus, &m.ErrorDetail,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}

	const resourcesQ = `
		SELECT id, message_id, resource_type, resource_json, created_at
		FROM fhir_resources
		WHERE message_id = $1
		ORDER BY id`

	rows, err := s.db.QueryContext(ctx, resourcesQ, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var resources []FHIRResource
	for rows.Next() {
		var r FHIRResource
		if err := rows.Scan(&r.ID, &r.MessageID, &r.ResourceType, &r.ResourceJSON, &r.CreatedAt); err != nil {
			return nil, nil, err
		}
		resources = append(resources, r)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return &m, resources, nil
}

func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *PostgresStore) GetPatientByID(ctx context.Context, id string) (*FHIRResource, error) {
	const q = `
		SELECT id, message_id, resource_type, resource_json, created_at
		FROM fhir_resources
		WHERE resource_type = 'Patient' AND resource_json->>'id' = $1
		ORDER BY created_at DESC
		LIMIT 1`

	var r FHIRResource
	err := s.db.QueryRowContext(ctx, q, id).Scan(&r.ID, &r.MessageID, &r.ResourceType, &r.ResourceJSON, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *PostgresStore) SearchPatientsByFamilyName(ctx context.Context, family string) ([]FHIRResource, error) {
	const q = `
		SELECT id, message_id, resource_type, resource_json, created_at
		FROM fhir_resources
		WHERE resource_type = 'Patient'
		  AND EXISTS (
		      SELECT 1 FROM jsonb_array_elements(resource_json->'name') AS n
		      WHERE n->>'family' ILIKE $1
		  )
		ORDER BY created_at DESC`

	return s.queryFHIRResources(ctx, q, "%"+family+"%")
}

func (s *PostgresStore) ListObservationsForPatient(ctx context.Context, patientID string) ([]FHIRResource, error) {
	const q = `
		SELECT o.id, o.message_id, o.resource_type, o.resource_json, o.created_at
		FROM fhir_resources o
		WHERE o.resource_type = 'Observation'
		  AND o.message_id IN (
		      SELECT message_id FROM fhir_resources
		      WHERE resource_type = 'Patient' AND resource_json->>'id' = $1
		  )
		ORDER BY o.id`

	return s.queryFHIRResources(ctx, q, patientID)
}

func (s *PostgresStore) queryFHIRResources(ctx context.Context, query string, args ...any) ([]FHIRResource, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resources := []FHIRResource{}
	for rows.Next() {
		var r FHIRResource
		if err := rows.Scan(&r.ID, &r.MessageID, &r.ResourceType, &r.ResourceJSON, &r.CreatedAt); err != nil {
			return nil, err
		}
		resources = append(resources, r)
	}
	return resources, rows.Err()
}
