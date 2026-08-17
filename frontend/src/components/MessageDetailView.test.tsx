import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import type { MessageDetail } from '../api/types'
import { MessageDetailView } from './MessageDetailView'

const successMessage: MessageDetail = {
  id: 1,
  raw_message: 'MSH|^~\\&|SENDER|FAC|RECEIVER|FAC|20260817100000||ADT^A01|MSG001|P|2.3\rPID|1||PT001||DOE^JOHN',
  message_type: 'ADT^A01',
  received_at: '2026-08-17T10:00:00Z',
  parse_status: 'success',
  fhir_resources: [
    {
      id: 1,
      message_id: 1,
      resource_type: 'Patient',
      resource_json: { resourceType: 'Patient', id: 'MSG001' },
      created_at: '2026-08-17T10:00:00Z',
    },
  ],
}

const failedMessage: MessageDetail = {
  id: 2,
  raw_message: 'MSH|^~\\&|SENDER|FAC|RECEIVER|FAC|20260817110000||ADT^A01|MSG002|P|2.3',
  message_type: 'ADT^A01',
  received_at: '2026-08-17T11:00:00Z',
  parse_status: 'failed',
  error_detail: 'hl7: missing required PV1 segment',
  fhir_resources: [],
}

describe('MessageDetailView', () => {
  it('renders the raw HL7 pane with one line per segment', () => {
    render(<MessageDetailView message={successMessage} />)

    expect(screen.getByText(/^MSH\|/)).toBeInTheDocument()
    expect(screen.getByText(/^PID\|/)).toBeInTheDocument()
  })

  it('renders the pretty-printed FHIR pane', () => {
    render(<MessageDetailView message={successMessage} />)

    expect(screen.getByText('Patient')).toBeInTheDocument()
    expect(screen.getByText(/"resourceType": "Patient"/)).toBeInTheDocument()
  })

  it('shows a success badge and no error banner for a successfully parsed message', () => {
    render(<MessageDetailView message={successMessage} />)

    expect(screen.getByText('Success')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('prominently shows the error detail and an empty FHIR pane for a failed-parse message', () => {
    render(<MessageDetailView message={failedMessage} />)

    expect(screen.getByText('Failed')).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('hl7: missing required PV1 segment')
    expect(screen.getByText(/no fhir resources were generated/i)).toBeInTheDocument()
  })

  it('does not show a raw/formatted JSON toggle when there are no FHIR resources', () => {
    render(<MessageDetailView message={failedMessage} />)

    expect(screen.queryByRole('button', { name: /view raw json/i })).not.toBeInTheDocument()
  })

  it('toggles the FHIR pane between pretty-printed and raw JSON', async () => {
    const user = userEvent.setup()
    render(<MessageDetailView message={successMessage} />)

    expect(screen.getByText(/"resourceType": "Patient"/)).toBeInTheDocument()
    expect(screen.queryByText('{"resourceType":"Patient","id":"MSG001"}')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /view raw json/i }))

    expect(screen.getByText('{"resourceType":"Patient","id":"MSG001"}')).toBeInTheDocument()
    expect(screen.queryByText(/"resourceType": "Patient"/)).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /view formatted json/i }))

    expect(screen.getByText(/"resourceType": "Patient"/)).toBeInTheDocument()
  })
})
