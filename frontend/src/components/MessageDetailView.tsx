import type { MessageDetail } from '../api/types'
import { formatReceivedAt } from '../utils/format'
import { splitHL7Segments } from '../utils/hl7'
import { StatusBadge } from './StatusBadge'

interface MessageDetailViewProps {
  message: MessageDetail
}

export function MessageDetailView({ message }: MessageDetailViewProps) {
  const segments = splitHL7Segments(message.raw_message)
  const isFailed = message.parse_status === 'failed'

  return (
    <div className="message-detail">
      <header className="message-detail__header">
        <h1>{message.message_type}</h1>
        <StatusBadge status={message.parse_status} />
        <span className="message-detail__received">{formatReceivedAt(message.received_at)}</span>
      </header>

      {isFailed && message.error_detail && (
        <div className="message-detail__error" role="alert">
          <strong>Parse error:</strong> {message.error_detail}
        </div>
      )}

      <section className="message-detail__section">
        <h2>Raw HL7v2</h2>
        <pre className="hl7-raw">
          {segments.map((segment, index) => (
            <div key={index} className="hl7-raw__line">
              {segment}
            </div>
          ))}
        </pre>
      </section>

      <section className="message-detail__section">
        <h2>Generated FHIR</h2>
        {message.fhir_resources.length === 0 ? (
          <p className="empty-state">No FHIR resources were generated for this message.</p>
        ) : (
          message.fhir_resources.map((resource) => (
            <div key={resource.id} className="fhir-resource">
              <h3>{resource.resource_type}</h3>
              <pre className="fhir-json">{JSON.stringify(resource.resource_json, null, 2)}</pre>
            </div>
          ))
        )}
      </section>
    </div>
  )
}
