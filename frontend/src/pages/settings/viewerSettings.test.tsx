import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import ServerSettingsPage from './ServerSettingsPage'
import SystemSettingsPage from './SystemSettingsPage'
import StorageSettingsPage from './StorageSettingsPage'
import CamerasSettingsPage from './CamerasSettingsPage'

afterEach(cleanup)

const mockFetch = vi.fn()
global.fetch = mockFetch

vi.mock('../../auth', () => ({
  getRole: vi.fn(() => 'viewer'),
  authHeaders: () => ({}),
  getToken: () => 'fake',
  clearToken: vi.fn(),
}))

vi.mock('../../hooks/useSettings', () => ({
  useSettings: () => ({ settings: null, reload: vi.fn() }),
}))

vi.mock('../../components/SettingsLayout', () => ({
  default: ({ children }: { children: React.ReactNode }) => <div data-testid="settings-layout">{children}</div>,
}))

vi.mock('../../components/SettingsSidebar', () => ({
  SettingsSidebar: () => null,
}))

vi.mock('../../components/AppLayout', () => ({
  default: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}))

describe('viewer — restricted pages', () => {
  beforeEach(() => {
    mockFetch.mockResolvedValue({ ok: false, status: 403 })
  })

  it('ServerSettingsPage shows Acesso restrito for viewer', () => {
    render(
      <MemoryRouter>
        <ServerSettingsPage />
      </MemoryRouter>
    )
    expect(screen.getAllByText('Acesso restrito.').length).toBeGreaterThan(0)
    expect(screen.queryByText('Carregando...')).toBeNull()
  })

  it('SystemSettingsPage shows Acesso restrito for viewer', () => {
    render(
      <MemoryRouter>
        <SystemSettingsPage />
      </MemoryRouter>
    )
    expect(screen.getAllByText('Acesso restrito.').length).toBeGreaterThan(0)
    expect(screen.queryByText('Carregando...')).toBeNull()
  })

  it('StorageSettingsPage shows Acesso restrito for viewer', () => {
    render(
      <MemoryRouter>
        <StorageSettingsPage />
      </MemoryRouter>
    )
    expect(screen.getAllByText('Acesso restrito.').length).toBeGreaterThan(0)
    expect(screen.queryByText('Carregando...')).toBeNull()
  })
})

describe('viewer — CamerasSettingsPage', () => {
  it('fetches from /api/cameras and shows camera list with badges', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => [
        { id: 'cam1', name: 'Hall', recording_enabled: true, motion: { enabled: true } },
        { id: 'cam2', name: 'Quintal', recording_enabled: false, motion: null },
      ],
    })

    render(
      <MemoryRouter initialEntries={['/settings/cameras']}>
        <Routes>
          <Route path="/settings/cameras" element={<CamerasSettingsPage />} />
        </Routes>
      </MemoryRouter>
    )

    await waitFor(() => expect(screen.getByText('Hall')).toBeTruthy())
    expect(screen.getByText('Quintal')).toBeTruthy()
    expect(screen.getByText('motion')).toBeTruthy()
    expect(screen.getByText('rec off')).toBeTruthy()
    expect(screen.queryByText(/nova câmera/i)).toBeNull()

    const calls = mockFetch.mock.calls.map((c: unknown[]) => c[0] as string)
    expect(calls.some(u => u === '/api/cameras')).toBe(true)
    expect(calls.every((u: string) => u !== '/api/settings/cameras')).toBe(true)
  })

  it('shows Nenhuma câmera disponível when list is empty', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => [],
    })

    render(
      <MemoryRouter initialEntries={['/settings/cameras']}>
        <Routes>
          <Route path="/settings/cameras" element={<CamerasSettingsPage />} />
        </Routes>
      </MemoryRouter>
    )

    await waitFor(() => {
      expect(screen.getByText('Nenhuma câmera disponível.')).toBeTruthy()
    })
  })
})
