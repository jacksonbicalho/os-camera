import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import SettingsLayout from './SettingsLayout'

vi.mock('./AppLayout', () => ({
  default: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}))
vi.mock('../auth', () => ({ getRole: () => 'admin' }))

afterEach(() => cleanup())

describe('SettingsLayout — link Estatísticas', () => {
  it('exibe um link "Estatísticas" apontando para /stats', () => {
    render(
      <MemoryRouter>
        <SettingsLayout><div /></SettingsLayout>
      </MemoryRouter>,
    )
    const link = screen.getByRole('link', { name: 'Estatísticas' })
    expect(link.getAttribute('href')).toBe('/stats')
  })
})
