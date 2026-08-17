import { useState, type FormEvent } from 'react'
import { getPatientById, listObservationsForPatient, searchPatientsByFamily } from '../api/fhir'
import type { Bundle } from '../api/types'

type SearchMode = 'patient-by-id' | 'patient-by-family' | 'observations-by-patient'

const MODE_LABELS: Record<SearchMode, string> = {
  'patient-by-id': 'Patient by ID',
  'patient-by-family': 'Patient by Family Name',
  'observations-by-patient': 'Observations by Patient ID',
}

const MODE_PLACEHOLDERS: Record<SearchMode, string> = {
  'patient-by-id': 'e.g. 123456',
  'patient-by-family': 'e.g. DOE',
  'observations-by-patient': 'e.g. 123457',
}

type SearchResult =
  | { kind: 'patient'; patient: unknown }
  | { kind: 'patient-not-found'; id: string }
  | { kind: 'bundle'; bundle: Bundle; emptyMessage: string }

export function FhirExplorerPage() {
  const [mode, setMode] = useState<SearchMode>('patient-by-id')
  const [query, setQuery] = useState('')
  const [result, setResult] = useState<SearchResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  function handleModeChange(newMode: SearchMode) {
    setMode(newMode)
    setResult(null)
    setError(null)
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const q = query.trim()
    if (!q || loading) return

    setLoading(true)
    setError(null)
    setResult(null)
    try {
      switch (mode) {
        case 'patient-by-id': {
          const patient = await getPatientById(q)
          setResult(patient === null ? { kind: 'patient-not-found', id: q } : { kind: 'patient', patient })
          break
        }
        case 'patient-by-family': {
          const bundle = await searchPatientsByFamily(q)
          setResult({ kind: 'bundle', bundle, emptyMessage: `No patients matched "${q}".` })
          break
        }
        case 'observations-by-patient': {
          const bundle = await listObservationsForPatient(q)
          setResult({
            kind: 'bundle',
            bundle,
            emptyMessage: `No observations found for patient "${q}" (or the patient doesn't exist — this endpoint can't tell the two apart).`,
          })
          break
        }
      }
    } catch {
      setError(`Failed to search (${MODE_LABELS[mode]}).`)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="page">
      <h1>FHIR Search</h1>
      <p className="converter-hint">Look up FHIR resources derived from previously ingested messages.</p>

      <section className="fhir-section">
        <form className="fhir-form" onSubmit={handleSubmit}>
          <select
            value={mode}
            onChange={(event) => handleModeChange(event.target.value as SearchMode)}
            aria-label="Search type"
          >
            {Object.entries(MODE_LABELS).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
          <input
            type="text"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={MODE_PLACEHOLDERS[mode]}
            aria-label="Search query"
          />
          <button type="submit" disabled={loading || !query.trim()}>
            {loading ? 'Searching…' : 'Search'}
          </button>
        </form>

        {error && (
          <p className="error-banner" role="alert">
            {error}
          </p>
        )}
        {result?.kind === 'patient-not-found' && (
          <p className="empty-state">No patient found with id "{result.id}".</p>
        )}
        {result?.kind === 'patient' && <pre className="fhir-json">{JSON.stringify(result.patient, null, 2)}</pre>}
        {result?.kind === 'bundle' &&
          (result.bundle.total === 0 ? (
            <p className="empty-state">{result.emptyMessage}</p>
          ) : (
            result.bundle.entry.map((entry, index) => (
              <pre key={index} className="fhir-json">
                {JSON.stringify(entry.resource, null, 2)}
              </pre>
            ))
          ))}
      </section>
    </div>
  )
}
