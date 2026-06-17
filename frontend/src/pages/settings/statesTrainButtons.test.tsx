import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
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
  { id: 2, name: 'Janela', classes: ['acesa', 'apagada'], trigger_interval_seconds: 3, threshold: 0.5, enabled: true, crop_x: 0.2, crop_y: 0.2, crop_w: 0.2, crop_h: 0.2, trigger_motion: false, min_consecutive: 2 },
]

const trainCalls: { url: string; body: string }[] = []

beforeEach(() => {
  trainCalls.length = 0
  vi.stubGlobal('fetch', vi.fn(async (url: unknown, opts: { method?: string; body?: string } = {}) => {
    const u = String(url)
    const method = opts.method ?? 'GET'
    if (method === 'POST' && u.includes('/train')) {
      trainCalls.push({ url: u, body: String(opts.body ?? '') })
      return new Response(JSON.stringify({ job_id: 'j1' }), { status: 200 })
    }
    if (u.endsWith('/classifiers')) {
      return new Response(JSON.stringify(classifiers), { status: 200 })
    }
    if (u.includes('/state')) {
      return new Response(JSON.stringify({ state: 'aberto' }), { status: 200 })
    }
    return new Response('{}', { status: 200 })
  }))
})

afterEach(() => { cleanup(); vi.unstubAllGlobals() })

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/settings/cameras/states/cam1']}>
      <Routes>
        <Route path="/settings/cameras/states/:id" element={<CameraStatesSettingsPage />} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('CameraStatesSettingsPage — botões de treino', () => {
  it('"Treinar agora" por linha dispara POST .../train com corpo vazio e mostra status', async () => {
    renderPage()
    await screen.findByText('Portão')

    const btn = screen.getAllByTitle(/Treinar agora/i)[0]
    fireEvent.click(btn)

    await waitFor(() => expect(trainCalls).toHaveLength(1))
    expect(trainCalls[0].url).toContain('/classifiers/1/train')
    expect(trainCalls[0].body).toBe('{}')
    await screen.findByText(/Treino de "Portão" iniciado/i)
  })

  it('"Treinar todos" dispara o treino de cada classificador', async () => {
    renderPage()
    await screen.findByText('Portão')

    fireEvent.click(screen.getByText('Treinar todos'))

    await waitFor(() => expect(trainCalls).toHaveLength(2))
    expect(trainCalls.map(c => c.url).join(' ')).toMatch(/\/classifiers\/1\/train/)
    expect(trainCalls.map(c => c.url).join(' ')).toMatch(/\/classifiers\/2\/train/)
    await screen.findByText(/Treino iniciado em 2/i)
  })
})
