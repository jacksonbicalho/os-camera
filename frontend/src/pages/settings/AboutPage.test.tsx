import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import AboutPage from './AboutPage'
import type { UpdateStatus } from '../../hooks/useUpdates'

afterEach(cleanup)

vi.mock('../../components/SettingsLayout', () => ({
  default: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}))

vi.mock('../../hooks/useSettings', () => ({
  useAbout: () => ({
    version: 'v1.3.0-dev',
    commit: 'abc',
    built_at: '2026-06-25',
    uptime_seconds: 10,
    go_version: 'go1.25',
  }),
}))

let mockStatus: UpdateStatus | null
const applyUpdate = vi.fn()
let mockRole: string

vi.mock('../../hooks/useUpdates', () => ({
  useUpdates: () => ({ status: mockStatus, loading: false, reload: vi.fn(), applyUpdate }),
}))

vi.mock('../../auth', () => ({
  getRole: () => mockRole,
}))

function renderPage() {
  return render(
    <MemoryRouter>
      <AboutPage />
    </MemoryRouter>
  )
}

const base: UpdateStatus = {
  current: 'v1.3.0-dev',
  latest: 'v1.4.0-dev',
  notes_md: '### Novidades\n- coisa nova',
  image: 'jacksonbicalho/os-camera:1.4.0-dev',
  min_supported: 'v0.0.0',
  update_available: true,
  apply_mode: 'self-replace',
  checked_at: '2026-06-25T00:00:00Z',
  error: '',
}

describe('AboutPage updates section', () => {
  it('mostra nova versão e aplica (self-replace)', async () => {
    mockRole = 'admin'
    mockStatus = { ...base }
    applyUpdate.mockResolvedValue({ ok: true })
    renderPage()

    expect(screen.getByText(/v1\.4\.0-dev/)).toBeTruthy()
    expect(screen.getByText(/coisa nova/)).toBeTruthy()

    fireEvent.click(screen.getByText('Atualizar agora'))
    expect(applyUpdate).toHaveBeenCalled()
    await waitFor(() => expect(screen.getByText(/reiniciar/i)).toBeTruthy())
  })

  it('docker: instruções sem botão', () => {
    mockRole = 'admin'
    mockStatus = { ...base, apply_mode: 'docker' }
    renderPage()

    expect(screen.getByText(/docker compose pull/)).toBeTruthy()
    expect(screen.queryByText('Atualizar agora')).toBeNull()
  })

  it('em dia: não renderiza a seção', () => {
    mockRole = 'admin'
    mockStatus = { ...base, update_available: false }
    renderPage()

    expect(screen.queryByText(/última versão/i)).toBeNull()
    expect(screen.queryByText(/Atualiza/i)).toBeNull()
    expect(screen.queryByText('Atualizar agora')).toBeNull()
  })

  it('erro na checagem: não renderiza a seção', () => {
    mockRole = 'admin'
    mockStatus = { ...base, update_available: false, error: 'boom' }
    renderPage()

    expect(screen.queryByText(/Atualiza/i)).toBeNull()
  })

  it('não-admin não vê a seção', () => {
    mockRole = 'viewer'
    mockStatus = null
    renderPage()

    expect(screen.queryByText(/Atualiza/i)).toBeNull()
  })
})
