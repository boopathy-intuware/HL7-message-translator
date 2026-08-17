import axios from 'axios'
import apiClient from './client'
import type { Bundle } from './types'

// Returns the raw Patient FHIR resource, or null if no patient with that
// id exists (GET /fhir/Patient/:id responds 404 in that case).
export async function getPatientById(id: string): Promise<unknown | null> {
  try {
    const { data } = await apiClient.get<unknown>(`/fhir/Patient/${id}`)
    return data
  } catch (err) {
    if (axios.isAxiosError(err) && err.response?.status === 404) return null
    throw err
  }
}

export async function searchPatientsByFamily(family: string): Promise<Bundle> {
  const { data } = await apiClient.get<Bundle>('/fhir/Patient', { params: { family } })
  return data
}

export async function listObservationsForPatient(patientId: string): Promise<Bundle> {
  const { data } = await apiClient.get<Bundle>('/fhir/Observation', { params: { patient: patientId } })
  return data
}
