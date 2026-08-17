import apiClient from './client'
import type { MessageDetail, MessageSummary } from './types'

export async function listMessages(): Promise<MessageSummary[]> {
  const { data } = await apiClient.get<MessageSummary[]>('/api/messages')
  return data
}

export async function getMessage(id: string | number): Promise<MessageDetail> {
  const { data } = await apiClient.get<MessageDetail>(`/api/messages/${id}`)
  return data
}

// Ingests a raw HL7v2 message and returns the HL7v2 ACK text. The ingest
// endpoint's response is the bare ACK (AA/AE) — it never reveals the new
// message's id, so callers that need it should follow up with
// findIngestedMessageId.
export async function ingestMessage(raw: string): Promise<string> {
  const { data } = await apiClient.post<string>('/api/hl7/messages', raw, {
    headers: { 'Content-Type': 'text/plain' },
  })
  return data
}

// Resolves the id of a just-ingested message by walking GET /api/messages
// (most-recently-received first) and fetching each candidate's full detail
// until one's raw_message exactly matches raw. This stays correct even
// when many messages are ingested concurrently — each caller matches its
// own exact text rather than assuming "the newest entry" is its own.
// Returns null if no match is found (e.g. the message somehow never
// persisted).
export async function findIngestedMessageId(raw: string): Promise<number | null> {
  const summaries = await listMessages()
  for (const summary of summaries) {
    const detail = await getMessage(summary.id)
    if (detail.raw_message === raw) {
      return detail.id
    }
  }
  return null
}
