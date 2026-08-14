package api

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"time"

	"hl7-message-translator/backend/store"
)

// fakeStore is an in-memory store.Store standing in for Postgres in
// handler unit tests. IDs are assigned sequentially starting at 1, and
// ListMessages returns most-recently-ingested first, mirroring
// PostgresStore's "ORDER BY received_at DESC".
type fakeStore struct {
	mu        sync.Mutex
	messages  []store.Message
	resources map[int64][]store.FHIRResource
	pingErr   error // set to simulate a database outage for /ready tests
}

func newFakeStore() *fakeStore {
	return &fakeStore{resources: make(map[int64][]store.FHIRResource)}
}

func (s *fakeStore) IngestMessage(ctx context.Context, m store.NewMessage, resources []store.NewFHIRResource) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := int64(len(s.messages) + 1)
	s.messages = append(s.messages, store.Message{
		ID:          id,
		RawMessage:  m.RawMessage,
		MessageType: m.MessageType,
		ReceivedAt:  time.Now(),
		ParseStatus: m.ParseStatus,
		ErrorDetail: m.ErrorDetail,
	})

	stored := make([]store.FHIRResource, len(resources))
	for i, r := range resources {
		stored[i] = store.FHIRResource{
			ID:           int64(i) + 1,
			MessageID:    id,
			ResourceType: r.ResourceType,
			ResourceJSON: r.ResourceJSON,
			CreatedAt:    time.Now(),
		}
	}
	s.resources[id] = stored

	return id, nil
}

func (s *fakeStore) ListMessages(ctx context.Context) ([]store.MessageSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]store.MessageSummary, 0, len(s.messages))
	for _, m := range slices.Backward(s.messages) {
		out = append(out, store.MessageSummary{
			ID:          m.ID,
			MessageType: m.MessageType,
			ReceivedAt:  m.ReceivedAt,
			ParseStatus: m.ParseStatus,
			ErrorDetail: m.ErrorDetail,
		})
	}
	return out, nil
}

func (s *fakeStore) GetMessage(ctx context.Context, id int64) (*store.Message, []store.FHIRResource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range s.messages {
		if m.ID == id {
			found := m
			return &found, s.resources[id], nil
		}
	}
	return nil, nil, store.ErrNotFound
}

func (s *fakeStore) Ping(ctx context.Context) error {
	return s.pingErr
}

// fakePatientFields is the subset of a Patient resource's JSON that
// fakeStore's FHIR read methods need to inspect.
type fakePatientFields struct {
	ID   string `json:"id"`
	Name []struct {
		Family string `json:"family"`
	} `json:"name"`
}

func (s *fakeStore) GetPatientByID(ctx context.Context, id string) (*store.FHIRResource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range slices.Backward(s.messages) {
		for _, r := range s.resources[m.ID] {
			if r.ResourceType != "Patient" {
				continue
			}
			var fields fakePatientFields
			if err := json.Unmarshal(r.ResourceJSON, &fields); err != nil {
				return nil, err
			}
			if fields.ID == id {
				found := r
				return &found, nil
			}
		}
	}
	return nil, store.ErrNotFound
}

func (s *fakeStore) SearchPatientsByFamilyName(ctx context.Context, family string) ([]store.FHIRResource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	family = strings.ToLower(family)
	matches := []store.FHIRResource{}
	for _, m := range slices.Backward(s.messages) {
		for _, r := range s.resources[m.ID] {
			if r.ResourceType != "Patient" {
				continue
			}
			var fields fakePatientFields
			if err := json.Unmarshal(r.ResourceJSON, &fields); err != nil {
				return nil, err
			}
			for _, n := range fields.Name {
				if strings.Contains(strings.ToLower(n.Family), family) {
					matches = append(matches, r)
					break
				}
			}
		}
	}
	return matches, nil
}

func (s *fakeStore) ListObservationsForPatient(ctx context.Context, patientID string) ([]store.FHIRResource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	matchingMessageIDs := make(map[int64]bool)
	for _, m := range s.messages {
		for _, r := range s.resources[m.ID] {
			if r.ResourceType != "Patient" {
				continue
			}
			var fields fakePatientFields
			if err := json.Unmarshal(r.ResourceJSON, &fields); err != nil {
				return nil, err
			}
			if fields.ID == patientID {
				matchingMessageIDs[m.ID] = true
			}
		}
	}

	observations := []store.FHIRResource{}
	for _, m := range s.messages {
		if !matchingMessageIDs[m.ID] {
			continue
		}
		for _, r := range s.resources[m.ID] {
			if r.ResourceType == "Observation" {
				observations = append(observations, r)
			}
		}
	}
	return observations, nil
}
