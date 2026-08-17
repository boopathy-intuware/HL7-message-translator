// Mirrors the JSON shapes produced by backend/store (store.go) and
// backend/api (handlers.go messageDetail).

export type ParseStatus = 'success' | 'failed'

export interface MessageSummary {
  id: number
  message_type: string
  received_at: string
  parse_status: ParseStatus
  error_detail?: string
}

export interface FHIRResource {
  id: number
  message_id: number
  resource_type: string
  resource_json: unknown
  created_at: string
}

export interface MessageDetail {
  id: number
  raw_message: string
  message_type: string
  received_at: string
  parse_status: ParseStatus
  error_detail?: string
  fhir_resources: FHIRResource[]
}
