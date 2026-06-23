import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom'
import RecordingsPage from './RecordingsPage'

vi.mock('../auth', () => ({
  authHeaders: () => ({}),
  getToken: () => 'fake',
  onUnauthorized: vi.fn(),
}))
vi.mock('../components/AppLayout', () => ({ default: ({ children }: { children: React.ReactNode }) => <div>{children}</div> }))
vi.mock('../components/DatePicker', () => ({ default: () => <div data-testid="datepicker" /> }))

const cameras = [{ id: 'cam1', name: 'Corredor' }, { id: 'cam2', name: 'Quintal' }]
const moments = [
  { camera_id: 'cam1', camera_name: 'Corredor', time: '2026-06-23T08:08:05Z', kind: 'state', label: 'aberto', category: 'estados', frame: '/recordings/state_history/1/x.jpg', score: 0.9 },
  { camera_id: 'cam2', camera_name: 'Quintal', time: '2026-06-23T07:00:00Z', kind: 'motion', label: 'pessoa', category: 'pessoa', frame: '20260623070000_motion.jpg', score: 0.5 },
]

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn((url: string) => {
    if (url.startsWith('/api/cameras')) return Promise.resolve({ status: 200, json: () => Promise.resolve(cameras) })
    if (url.startsWith('/api/moments')) return Promise.resolve({ status: 200, json: () => Promise.resolve({ moments, total: 2, hasMore: false }) })
    return Promise.resolve({ status: 404, json: () => Promise.resolve({}) })
  }))
})
afterEach(() => { cleanup(); vi.unstubAllGlobals() })

function LocationProbe() {
  const l = useLocation()
  return <div data-testid="loc">{l.pathname}|{JSON.stringify(l.state)}</div>
}

describe('RecordingsPage', () => {
  it('lista momentos com nome da câmera e clique abre a câmera no instante', async () => {
    render(
      <MemoryRouter initialEntries={['/recordings']}>
        <Routes>
          <Route path="/recordings" element={<RecordingsPage />} />
          <Route path="/cameras/:id" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>,
    )
    // espera os cards renderizarem (por id, pois o nome também aparece nos chips de filtro)
    const card0 = await waitFor(() => {
      const el = document.getElementById('moment-0')
      if (!el) throw new Error('card não renderizou')
      return el
    })
    expect(card0.textContent).toContain('Corredor')
    expect(document.getElementById('moment-1')?.textContent).toContain('Quintal')

    fireEvent.click(card0)
    const loc = await screen.findByTestId('loc')
    expect(loc.textContent).toContain('/cameras/cam1')
    expect(loc.textContent).toContain('2026-06-23T08:08:05Z')
  })
})
