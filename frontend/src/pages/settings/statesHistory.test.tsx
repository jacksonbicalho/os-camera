import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom'
import CameraStatesSettingsPage from './CameraStatesSettingsPage'

vi.mock('../../auth', () => ({
  getRole: () => 'admin',
  authHeaders: () => ({}),
  getToken: () => 'fake',
  onUnauthorized: vi.fn(),
}))

vi.mock('../../components/SettingsLayout', () => ({
  default: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}))
vi.mock('../../components/CameraSettingsTabs', () => ({ default: () => <div /> }))

const classifiers = [
  { id: 1, name: 'Portão', classes: ['aberto', 'fechado'], trigger_interval_seconds: 10, threshold: 0.8, enabled: true, crop_x: 0.1, crop_y: 0.1, crop_w: 0.3, crop_h: 0.3, trigger_motion: false, min_consecutive: 3 },
]

const history = [
  { state: 'aberto', confidence: 0.9, changed_at: '2026-06-18T10:00:00Z', frame: '/recordings/state_history/1/a.jpg', recording_available: true },
  { state: 'fechado', confidence: 0.8, changed_at: '2026-06-10T09:00:00Z', frame: '/recordings/state_history/1/b.jpg', recording_available: false },
]

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn(async (url: unknown) => {
    const u = String(url)
    if (u.includes('/history')) return new Response(JSON.stringify(history), { status: 200 })
    if (u.endsWith('/classifiers')) return new Response(JSON.stringify(classifiers), { status: 200 })
    if (u.includes('/state')) return new Response(JSON.stringify({ state: 'aberto' }), { status: 200 })
    return new Response('{}', { status: 200 })
  }))
})

afterEach(() => { cleanup(); vi.unstubAllGlobals() })

// Mostra o eventTime recebido por navegação, para verificar o "Ver na gravação".
function CameraProbe() {
  const loc = useLocation() as { state?: { eventTime?: string } }
  return <div data-testid="camera-probe">{loc.state?.eventTime ?? ''}</div>
}

function renderPage(initial = '/settings/cameras/states/cam1') {
  return render(
    <MemoryRouter initialEntries={[initial]}>
      <Routes>
        <Route path="/settings/cameras/states/:id" element={<CameraStatesSettingsPage />} />
        <Route path="/cameras/:id" element={<CameraProbe />} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('CameraStatesSettingsPage — histórico', () => {
  it('botão Histórico abre o grid; thumb abre lightbox; "Ver na gravação" navega no changed_at', async () => {
    renderPage()
    await screen.findByText('Portão')

    fireEvent.click(document.getElementById('state-history-1')!)
    await screen.findByText('Histórico — Portão')

    // clica no 1º thumb (recording_available=true) → lightbox
    fireEvent.click(document.getElementById('state-history-thumb-0')!)
    const watch = document.getElementById('state-history-watch') as HTMLButtonElement
    expect(watch).toBeTruthy()
    expect(watch.disabled).toBe(false)

    fireEvent.click(watch)
    expect(screen.getByTestId('camera-probe').textContent).toBe('2026-06-18T10:00:00Z')
  })

  it('thumb com gravação expirada desabilita "Ver na gravação"', async () => {
    renderPage()
    await screen.findByText('Portão')
    fireEvent.click(document.getElementById('state-history-1')!)
    await screen.findByText('Histórico — Portão')

    fireEvent.click(document.getElementById('state-history-thumb-1')!)
    const watch = document.getElementById('state-history-watch') as HTMLButtonElement
    expect(watch.disabled).toBe(true)
    expect(watch.textContent).toContain('Gravação expirada')
  })

  it('deep-link ?history={cid} abre direto a view de Histórico', async () => {
    renderPage('/settings/cameras/states/cam1?history=1')
    await screen.findByText('Histórico — Portão')
  })
})
