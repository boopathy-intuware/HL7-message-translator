import type { ParseStatus } from '../api/types'
import { AlertCircleIcon, CheckCircleIcon } from './icons'

interface StatusBadgeProps {
  status: ParseStatus
}

export function StatusBadge({ status }: StatusBadgeProps) {
  const isSuccess = status === 'success'
  return (
    <span className={`status-badge status-badge--${isSuccess ? 'success' : 'failed'}`}>
      {isSuccess ? <CheckCircleIcon size={14} /> : <AlertCircleIcon size={14} />}
      {isSuccess ? 'Success' : 'Failed'}
    </span>
  )
}
