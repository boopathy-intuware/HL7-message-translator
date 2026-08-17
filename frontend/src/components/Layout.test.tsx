import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { Layout } from './Layout'

describe('Layout', () => {
  it('renders navigation links to the converter, inbox, and FHIR search pages', () => {
    render(
      <MemoryRouter>
        <Layout />
      </MemoryRouter>,
    )

    expect(screen.getByRole('link', { name: 'Convert' })).toHaveAttribute('href', '/')
    expect(screen.getByRole('link', { name: 'Inbox' })).toHaveAttribute('href', '/inbox')
    expect(screen.getByRole('link', { name: 'FHIR Search' })).toHaveAttribute('href', '/fhir')
  })
})
