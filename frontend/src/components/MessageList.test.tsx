import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import type { MessageSummary } from '../api/types'
import { MessageList } from './MessageList'

const messages: MessageSummary[] = [
  {
    id: 1,
    message_type: 'ADT^A01',
    received_at: '2026-08-17T10:00:00Z',
    parse_status: 'success',
  },
  {
    id: 2,
    message_type: 'ORU^R01',
    received_at: '2026-08-17T11:00:00Z',
    parse_status: 'failed',
    error_detail: 'PV1 missing patient class',
  },
]

function renderList(list: MessageSummary[]) {
  return render(
    <MemoryRouter>
      <MessageList messages={list} />
    </MemoryRouter>,
  )
}

describe('MessageList', () => {
  it('renders one row per message with its type', () => {
    renderList(messages)

    expect(screen.getByText('ADT^A01')).toBeInTheDocument()
    expect(screen.getByText('ORU^R01')).toBeInTheDocument()
    expect(screen.getAllByRole('row')).toHaveLength(messages.length + 1) // + header row
  })

  it('shows success and failed statuses with distinct labels', () => {
    renderList(messages)

    expect(screen.getByText('Success')).toBeInTheDocument()
    expect(screen.getByText('Failed')).toBeInTheDocument()
  })

  it('links each row to its message detail page', () => {
    renderList(messages)

    expect(screen.getByRole('link', { name: 'ADT^A01' })).toHaveAttribute('href', '/messages/1')
    expect(screen.getByRole('link', { name: 'ORU^R01' })).toHaveAttribute('href', '/messages/2')
  })

  it('shows an empty state when there are no messages', () => {
    renderList([])

    expect(screen.getByText(/no messages have been ingested/i)).toBeInTheDocument()
  })
})