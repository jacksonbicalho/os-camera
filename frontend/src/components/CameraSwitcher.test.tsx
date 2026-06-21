import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import CameraSwitcher from './CameraSwitcher'

const navigate = vi.fn()

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return { ...actual, useNavigate: () => navigate, useParams: () => ({ id: 'cam1' }) }
})

vi.mock('../auth', () => ({ authHeaders: () => ({}), onUnauthorized: vi.fn() }))

afterEach(() => { cleanup(); navigate.mockClear() })

describe('CameraSwitcher', () => {
  it('abre o dropdown, lista as câmeras e navega ao escolher', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 200,
      json: async () => [{ id: 'cam1', name: 'Frente' }, { id: 'cam2', name: 'Fundos' }],
    }) as unknown as typeof fetch

    render(<MemoryRouter><CameraSwitcher /></MemoryRouter>)
    expect(document.getElementById('camera-switcher')).toBeTruthy()

    fireEvent.click(document.getElementById('camera-switcher')!)
    await waitFor(() => expect(screen.getByText('Fundos')).toBeTruthy())

    fireEvent.click(screen.getByText('Fundos'))
    expect(navigate).toHaveBeenCalledWith('/camera/live/cam2', expect.objectContaining({ replace: true }))
  })
})
