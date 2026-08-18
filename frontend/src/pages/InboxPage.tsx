import { useEffect, useState } from 'react'
import { listMessages } from '../api/messages'
import type { MessageSummary } from '../api/types'
import { AlertCircleIcon } from '../components/icons'
import { MessageList } from '../components/MessageList'

export function InboxPage() {
  const [messages, setMessages] = useState<MessageSummary[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    listMessages()
      .then((data) => {
        if (!cancelled) setMessages(data)
      })
      .catch(() => {
        if (!cancelled) setError('Failed to load messages.')
      })
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <div className="page">
      <h1>Inbox</h1>
      {error && (
        <p className="error-banner" role="alert">
          <AlertCircleIcon size={16} />
          {error}
        </p>
      )}
      {!error && messages === null && <p>Loading messages…</p>}
      {messages !== null && <MessageList messages={messages} />}
    </div>
  )
}
