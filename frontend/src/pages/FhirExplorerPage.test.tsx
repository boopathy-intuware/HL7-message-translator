import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getPatientById, listObservationsForPatient, searchPatientsByFamily } from '../api/fhir'
import type { Bundle } from '../api/types'
import { FhirExplorerPage } from './FhirExplorerPage'

vi.mock('../api/fhir')

async function search(user: ReturnType<typeof userEvent.setup>, mode: string, query: string) {
  await user.selectOptions(screen.getByLabelText('Search type'), mode)
  await user.type(screen.getByLabelText('Search query'), query)
  await user.click(screen.getByRole('button', { name: /search/i }))
}

describe('FhirExplorerPage', () => {
  beforeEach(() => {
    vi.mocked(getPatientById).mockReset()
    vi.mocked(searchPatientsByFamily).mockReset()
    vi.mocked(listObservationsForPatient).mockReset()
  })

  it('defaults to Patient by ID mode', () => {
    render(<FhirExplorerPage />)

    expect(screen.getByLabelText('Search type')).toHaveValue('patient-by-id')
  })

  describe('Patient by ID', () => {
    it('renders the found patient as JSON', async () => {
      vi.mocked(getPatientById).mockResolvedValue({ resourceType: 'Patient', id: '123456' })
      const user = userEvent.setup()
      render(<FhirExplorerPage />)

      await search(user, 'Patient by ID', '123456')

      expect(await screen.findByText(/"id": "123456"/)).toBeInTheDocument()
    })

    it('shows a not-found message when the patient does not exist', async () => {
      vi.mocked(getPatientById).mockResolvedValue(null)
      const user = userEvent.setup()
      render(<FhirExplorerPage />)

      await search(user, 'Patient by ID', 'no-such-id')

      expect(await screen.findByText(/no patient found with id "no-such-id"/i)).toBeInTheDocument()
    })

    it('shows an error banner when the lookup fails', async () => {
      vi.mocked(getPatientById).mockRejectedValue(new Error('network error'))
      const user = userEvent.setup()
      render(<FhirExplorerPage />)

      await search(user, 'Patient by ID', '123456')

      expect(await screen.findByRole('alert')).toHaveTextContent(/failed to search/i)
    })
  })

  describe('Patient by Family Name', () => {
    const bundle: Bundle = {
      resourceType: 'Bundle',
      type: 'searchset',
      total: 2,
      entry: [
        { resource: { resourceType: 'Patient', id: '123456' } },
        { resource: { resourceType: 'Patient', id: '123457' } },
      ],
    }

    it('renders each matching patient as JSON', async () => {
      vi.mocked(searchPatientsByFamily).mockResolvedValue(bundle)
      const user = userEvent.setup()
      render(<FhirExplorerPage />)

      await search(user, 'Patient by Family Name', 'DOE')

      expect(await screen.findByText(/"id": "123456"/)).toBeInTheDocument()
      expect(screen.getByText(/"id": "123457"/)).toBeInTheDocument()
    })

    it('shows an empty state when nothing matches', async () => {
      vi.mocked(searchPatientsByFamily).mockResolvedValue({
        resourceType: 'Bundle',
        type: 'searchset',
        total: 0,
        entry: [],
      })
      const user = userEvent.setup()
      render(<FhirExplorerPage />)

      await search(user, 'Patient by Family Name', 'NOSUCHNAME')

      expect(await screen.findByText(/no patients matched "NOSUCHNAME"/i)).toBeInTheDocument()
    })
  })

  describe('Observations by Patient ID', () => {
    it('renders each observation as JSON', async () => {
      vi.mocked(listObservationsForPatient).mockResolvedValue({
        resourceType: 'Bundle',
        type: 'searchset',
        total: 1,
        entry: [{ resource: { resourceType: 'Observation', id: 'MSG00002-1' } }],
      })
      const user = userEvent.setup()
      render(<FhirExplorerPage />)

      await search(user, 'Observations by Patient ID', '123457')

      expect(await screen.findByText(/"id": "MSG00002-1"/)).toBeInTheDocument()
    })

    it('shows an empty state when there are no observations', async () => {
      vi.mocked(listObservationsForPatient).mockResolvedValue({
        resourceType: 'Bundle',
        type: 'searchset',
        total: 0,
        entry: [],
      })
      const user = userEvent.setup()
      render(<FhirExplorerPage />)

      await search(user, 'Observations by Patient ID', '123456')

      expect(await screen.findByText(/no observations found for patient "123456"/i)).toBeInTheDocument()
    })
  })

  it('clears the previous result when the search type changes', async () => {
    vi.mocked(getPatientById).mockResolvedValue({ resourceType: 'Patient', id: '123456' })
    const user = userEvent.setup()
    render(<FhirExplorerPage />)

    await search(user, 'Patient by ID', '123456')
    expect(await screen.findByText(/"id": "123456"/)).toBeInTheDocument()

    await user.selectOptions(screen.getByLabelText('Search type'), 'Patient by Family Name')

    expect(screen.queryByText(/"id": "123456"/)).not.toBeInTheDocument()
  })
})
