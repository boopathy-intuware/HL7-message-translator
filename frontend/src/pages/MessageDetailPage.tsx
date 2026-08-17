import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getMessage } from '../api/messages'
import type { MessageDetail } from '../api/types'
import { MessageDetailView } from '../components/MessageDetailView'

export function MessageDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [message, setMessage] = useState<MessageDetail | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!id) return
    let cancelled = false
    setMessage(null)
    setError(null)
    getMessage(id)
      .then((data) => {
        if (!cancelled) setMessage(data)
      })
      .catch(() => {
        if (!cancelled) setError('Failed to load message.')
      })
    return () => {
      cancelled = true
    }
  }, [id])

  return (
    <div className="page">
      <Link to="/inbox" className="back-link">
        ← Back to inbox
      </Link>
      {error && (
        <p className="error-banner" role="alert">
          {error}
        </p>
      )}
      {!error && !message && <p>Loading message…</p>}
      {message && <MessageDetailView message={message} />}
    </div>
  )
}
