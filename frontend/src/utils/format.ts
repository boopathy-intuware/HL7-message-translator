export function formatReceivedAt(isoTimestamp: string): string {
  return new Date(isoTimestamp).toLocaleString()
}
