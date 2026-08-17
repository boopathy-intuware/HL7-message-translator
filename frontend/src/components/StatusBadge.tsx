import type { ParseStatus } from '../api/types'

interface StatusBadgeProps {
  status: ParseStatus
}

export function StatusBadge({ status }: StatusBadgeProps) {
  const isSuccess = status === 'success'
  return (
    <span className={`status-badge status-badge--${isSuccess ? 'success' : 'failed'}`}>
      {isSuccess ? 'Success' : 'Failed'}
    </span>
  )
}
