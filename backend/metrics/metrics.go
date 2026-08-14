// Package metrics defines the Prometheus collectors exposed at /metrics.
package metrics

import "github.com/prometheus/client_golang/prometheus"

// Metrics holds the collectors the HL7 ingest pipeline reports against.
// All are labeled by message_type (e.g. "ADT^A01") so ingestion volume,
// failures, and latency can be broken down per message type.
type Metrics struct {
	MessagesIngested   *prometheus.CounterVec
	ParseFailures      *prometheus.CounterVec
	ProcessingDuration *prometheus.HistogramVec
}

// New creates the ingest pipeline's collectors and registers them against
// reg. Callers should pass a fresh prometheus.NewRegistry() in tests —
// registering the same collector names twice against a shared registry
// (e.g. prometheus.DefaultRegisterer across multiple tests) panics.
func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		MessagesIngested: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "hl7_messages_ingested_total",
			Help: "Total number of HL7v2 messages ingested via POST /api/hl7/messages.",
		}, []string{"message_type"}),
		ParseFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "hl7_parse_failures_total",
			Help: "Total number of ingested HL7v2 messages that failed to parse or map to FHIR.",
		}, []string{"message_type"}),
		ProcessingDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "hl7_processing_duration_seconds",
			Help:    "Time to parse, map, and persist a single ingested HL7v2 message.",
			Buckets: prometheus.DefBuckets,
		}, []string{"message_type"}),
	}

	reg.MustRegister(m.MessagesIngested, m.ParseFailures, m.ProcessingDuration)
	return m
}
