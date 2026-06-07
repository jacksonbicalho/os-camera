import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import SystemSettingsPage from './SystemSettingsPage'
import type { Settings } from '../../hooks/useSettings'

afterEach(cleanup)

vi.mock('../../auth', () => ({
  getRole: vi.fn(() => 'admin'),
  authHeaders: () => ({}),
  getToken: () => 'fake',
  clearToken: vi.fn(),
}))

vi.mock('../../components/SettingsLayout', () => ({
  default: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}))

let mockSettings: Settings
vi.mock('../../hooks/useSettings', () => ({
  useSettings: () => ({ settings: mockSettings, reload: vi.fn() }),
}))

function baseSettings(over: Partial<Settings['log']>): Settings {
  return {
    timezone: 'UTC',
    debug: false,
    log: { output: 'stdout', path: '', max_size_mb: 50, max_age_days: 30, max_backups: 10, compress: true, ...over },
    server: { port: 8080, segments_path: '', recordings_path: '', username: 'admin' },
    storage: { path: '', with_motion_minutes: 0, without_motion_minutes: 0, interval_minutes: 0, max_size_gb: 0, warn_percent: 0 },
    defaults: { chunk_duration: '5m', reconnect_interval: '10s' },
    cameras: [],
  }
}

describe('SystemSettingsPage — log rotation', () => {
  it('shows rotation fields when output is file', () => {
    mockSettings = baseSettings({ output: 'file', path: '/var/log/camera', max_size_mb: 25, max_age_days: 7, max_backups: 3, compress: false })
    render(
      <MemoryRouter>
        <SystemSettingsPage />
      </MemoryRouter>
    )
    expect(screen.getByText('25 MB')).toBeTruthy()
    expect(screen.getByText('7 dias')).toBeTruthy()
    expect(screen.getByText('3')).toBeTruthy()
    expect(screen.getByText('desativada')).toBeTruthy()
  })

  it('hides rotation fields when output is stdout', () => {
    mockSettings = baseSettings({ output: 'stdout' })
    render(
      <MemoryRouter>
        <SystemSettingsPage />
      </MemoryRouter>
    )
    expect(screen.queryByText('Rotaciona em')).toBeNull()
    expect(screen.queryByText('Compressão')).toBeNull()
  })
})
