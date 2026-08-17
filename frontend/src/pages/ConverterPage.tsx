import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { findIngestedMessageId, ingestMessage } from '../api/messages'

const PLACEHOLDER = 'MSH|^~\\&|SENDING_APP|SENDING_FAC|RECEIVING_APP|RECEIVING_FAC|20260817120000||ADT^A01|MSG00001|P|2.3\nPID|1||PATID001||DOE^JANE||19800101|F\nPV1|1|I|WARD1^101^A'

export function ConverterPage() {
  const [raw, setRaw] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!raw.trim() || submitting) return

    setSubmitting(true)
    setError(null)
    try {
      await ingestMessage(raw)
      const messageId = await findIngestedMessageId(raw)
      if (messageId === null) {
        setError('The message was ingested, but its detail view could not be located. Check the inbox.')
        return
      }
      navigate(`/messages/${messageId}`)
    } catch {
      setError('Failed to ingest message.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="page">
      <h1>HL7v2 → FHIR Converter</h1>
      <p className="converter-hint">
        Paste a raw HL7v2 message below and ingest it. You'll be taken to its detail view whether it parses
        successfully or not.
      </p>
      <form className="converter-form" onSubmit={handleSubmit}>
        <textarea
          className="converter-textarea"
          value={raw}
          onChange={(event) => setRaw(event.target.value)}
          placeholder={PLACEHOLDER}
          rows={14}
          spellCheck={false}
          aria-label="Raw HL7v2 message"
        />
        {error && (
          <p className="error-banner" role="alert">
            {error}
          </p>
        )}
        <button type="submit" disabled={submitting || !raw.trim()}>
          {submitting ? 'Ingesting…' : 'Ingest Message'}
        </button>
      </form>
    </div>
  )
}