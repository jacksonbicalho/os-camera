import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route, useLocation, useNavigate } from 'react-router-dom'
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

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn(async (url: unknown) => {
    const u = String(url)
    if (u.includes('/history')) return new Response('[]', { status: 200 })
    if (u.endsWith('/classifiers')) return new Response(JSON.stringify(classifiers), { status: 200 })
    if (u.includes('/state')) return new Response(JSON.stringify({ state: 'aberto' }), { status: 200 })
    return new Response('[]', { status: 200 })
  }))
})
afterEach(() => { cleanup(); vi.unstubAllGlobals() })

function LocationSearch() {
  const loc = useLocation()
  return <div data-testid="search">{loc.search}</div>
}
function GoNoParam() {
  const nav = useNavigate()
  return <button id="go-no-param" onClick={() => nav('/settings/cameras/states/cam1')}>x</button>
}

function renderAt(entry: string) {
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <Routes>
        <Route
          path="/settings/cameras/states/:id"
          element={<><CameraStatesSettingsPage /><LocationSearch /><GoNoParam /></>}
        />
      </Routes>
    </MemoryRouter>,
  )
}

const el = (id: string) => document.getElementById(id)

describe('CameraStatesSettingsPage — Histórico reflete na URL', () => {
  it('abrir pelo botão da lista escreve ?history={cid} na URL', async () => {
    renderAt('/settings/cameras/states/cam1')
    await screen.findByText('Portão')

    fireEvent.click(el('state-history-1')!)

    await waitFor(() => expect(screen.getByTestId('search').textContent).toContain('history=1'))
  })

  it('remover o param ?history fecha a view de Histórico', async () => {
    renderAt('/settings/cameras/states/cam1?history=1')
    // deep-link abre a view (botão Voltar do Histórico aparece)
    await screen.findByText('← Voltar')

    // remover o param (simula Voltar do navegador) → fecha a view
    fireEvent.click(el('go-no-param')!)
    await waitFor(() => expect(el('state-history-back')).toBeNull())
  })
})
