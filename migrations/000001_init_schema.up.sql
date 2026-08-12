CREATE TABLE messages (
    id            BIGSERIAL PRIMARY KEY,
    raw_message   TEXT NOT NULL,
    message_type  TEXT NOT NULL,
    received_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    parse_status  TEXT NOT NULL,
    error_detail  TEXT
);

CREATE TABLE fhir_resources (
    id             BIGSERIAL PRIMARY KEY,
    message_id     BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    resource_type  TEXT NOT NULL,
    resource_json  JSONB NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_fhir_resources_message_id ON fhir_resources(message_id);
