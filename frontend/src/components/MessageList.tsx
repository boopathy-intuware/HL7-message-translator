import { Link, useNavigate } from 'react-router-dom'
import type { MessageSummary } from '../api/types'
import { formatReceivedAt } from '../utils/format'
import { StatusBadge } from './StatusBadge'

interface MessageListProps {
  messages: MessageSummary[]
}

export function MessageList({ messages }: MessageListProps) {
  const navigate = useNavigate()

  if (messages.length === 0) {
    return <p className="empty-state">No messages have been ingested yet.</p>
  }

  return (
    <table className="message-table">
      <thead>
        <tr>
          <th>Type</th>
          <th>Received</th>
          <th>Status</th>
        </tr>
      </thead>
      <tbody>
        {messages.map((message) => (
          <tr
            key={message.id}
            className="message-row"
            onClick={() => navigate(`/messages/${message.id}`)}
          >
            <td>
              <Link to={`/messages/${message.id}`}>{message.message_type}</Link>
            </td>
            <td>{formatReceivedAt(message.received_at)}</td>
            <td>
              <StatusBadge status={message.parse_status} />
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
