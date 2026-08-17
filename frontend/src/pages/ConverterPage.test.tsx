import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { findIngestedMessageId, ingestMessage } from '../api/messages'
import { ConverterPage } from './ConverterPage'

vi.mock('../api/messages')

function renderConverter() {
  return render(
    <MemoryRouter initialEntries={['/']}>
      <Routes>
        <Route path="/" element={<ConverterPage />} />
        <Route path="/messages/:id" element={<div>Detail page for message</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('ConverterPage', () => {
  beforeEach(() => {
    vi.mocked(ingestMessage).mockReset()
    vi.mocked(findIngestedMessageId).mockReset()
  })

  it('disables the submit button until a message is entered', () => {
    renderConverter()

    expect(screen.getByRole('button', { name: /ingest message/i })).toBeDisabled()
  })

  it('redirects to the detail view of the ingested message on success', async () => {
    vi.mocked(ingestMessage).mockResolvedValue('MSH|^~\\&|...|MSA|AA|MSG001')
    vi.mocked(findIngestedMessageId).mockResolvedValue(42)
    const user = userEvent.setup()
    renderConverter()

    await user.type(
      screen.getByLabelText(/raw hl7v2 message/i),
      'MSH|^~\\&|A|B|C|D|20260101000000||ADT^A01|1|P|2.3',
    )
    await user.click(screen.getByRole('button', { name: /ingest message/i }))

    await waitFor(() => {
      expect(screen.getByText('Detail page for message')).toBeInTheDocument()
    })
  })

  it('shows an error message when ingestion fails', async () => {
    vi.mocked(ingestMessage).mockRejectedValue(new Error('network error'))
    const user = userEvent.setup()
    renderConverter()

    await user.type(
      screen.getByLabelText(/raw hl7v2 message/i),
      'MSH|^~\\&|A|B|C|D|20260101000000||ADT^A01|1|P|2.3',
    )
    await user.click(screen.getByRole('button', { name: /ingest message/i }))

    expect(await screen.findByRole('alert')).toHaveTextContent(/failed to ingest message/i)
  })

  it('shows an error message when the ingested message cannot be located afterward', async () => {
    vi.mocked(ingestMessage).mockResolvedValue('MSH|^~\\&|...|MSA|AA|MSG001')
    vi.mocked(findIngestedMessageId).mockResolvedValue(null)
    const user = userEvent.setup()
    renderConverter()

    await user.type(
      screen.getByLabelText(/raw hl7v2 message/i),
      'MSH|^~\\&|A|B|C|D|20260101000000||ADT^A01|1|P|2.3',
    )
    await user.click(screen.getByRole('button', { name: /ingest message/i }))

    expect(await screen.findByRole('alert')).toHaveTextContent(/could not be located/i)
  })
})
