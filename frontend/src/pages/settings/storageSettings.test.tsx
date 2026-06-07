import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import StorageSettingsPage from './StorageSettingsPage'

afterEach(cleanup)

vi.mock('../../auth', () => ({
  getRole: vi.fn(() => 'admin'),
  authHeaders: () => ({}),
  getToken: () => 'fake',
  clearToken: vi.fn(),
}))

vi.mock('../../hooks/useSettings', () => ({
  useSettings: () => ({
    settings: {
      storage: {
        with_motion_minutes: 1440,
        without_motion_minutes: 60,
        interval_minutes: 60,
        max_size_gb: 0,
        warn_percent: 90,
      },
    },
    reload: vi.fn(),
  }),
}))

vi.mock('../../components/SettingsLayout', () => ({
  default: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}))

vi.mock('../../components/ConfirmDialog', () => ({
  default: () => null,
}))

const mockFetch = vi.fn()
global.fetch = mockFetch

const drive1 = { id: 'drive-1', name: 'Backup S3', type: 's3', endpoint: 'https://s3.example.com', bucket: 'cam', region: 'us-east-1', prefix: '' }
const drive2 = { id: 'drive-2', name: 'Offsite', type: 's3', endpoint: 'https://s3.example.com', bucket: 'cam', region: 'us-east-1', prefix: '' }

function putCalls() {
  return mockFetch.mock.calls.filter(
    (c: unknown[]) => (c[0] as string).includes('/api/retention/') && (c[1] as RequestInit)?.method === 'PUT'
  )
}

describe('retention destination — unified dropdown (Apagar + drives)', () => {
  it('lists "Apagar" plus every registered drive as options', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url === '/api/drives') return Promise.resolve({ ok: true, json: async () => [drive1, drive2] })
      if (url === '/api/retention') return Promise.resolve({ ok: true, json: async () => [] })
      return Promise.resolve({ ok: true, json: async () => ({}) })
    })

    render(<MemoryRouter><StorageSettingsPage /></MemoryRouter>)

    const destSelect = await waitFor(() => {
      const s = screen.getAllByRole('combobox').find(el => (el as HTMLSelectElement).value === 'delete')
      expect(s).toBeTruthy()
      return s as HTMLSelectElement
    })

    const opts = within(destSelect).getAllByRole('option').map(o => o.textContent)
    expect(opts).toContain('Apagar')
    expect(opts).toContain('Backup S3')
    expect(opts).toContain('Offsite')
  })

  it('selecting a drive sends send_to_drive + drive_id', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url === '/api/drives') return Promise.resolve({ ok: true, json: async () => [drive1, drive2] })
      if (url === '/api/retention') return Promise.resolve({ ok: true, json: async () => [] })
      return Promise.resolve({ ok: true, json: async () => ({}) })
    })

    render(<MemoryRouter><StorageSettingsPage /></MemoryRouter>)

    const destSelect = await waitFor(() => {
      const s = screen.getAllByRole('combobox').find(el => (el as HTMLSelectElement).value === 'delete')
      expect(s).toBeTruthy()
      return s as HTMLSelectElement
    })

    mockFetch.mockClear()
    mockFetch.mockResolvedValue({ ok: true, json: async () => [] })
    fireEvent.change(destSelect, { target: { value: 'drive:drive-1' } })

    await waitFor(() => {
      const calls = putCalls()
      expect(calls.length).toBe(1)
      const body = JSON.parse((calls[0][1] as RequestInit).body as string)
      expect(body.action).toBe('send_to_drive')
      expect(body.drive_id).toBe('drive-1')
    })
  })

  it('selecting "Apagar" sends action delete', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url === '/api/drives') return Promise.resolve({ ok: true, json: async () => [drive1, drive2] })
      if (url === '/api/retention') return Promise.resolve({ ok: true, json: async () => [
        { category: 'with_motion', action: 'send_to_drive', drive_id: 'drive-2' },
      ] })
      return Promise.resolve({ ok: true, json: async () => ({}) })
    })

    render(<MemoryRouter><StorageSettingsPage /></MemoryRouter>)

    const driveSelect = await waitFor(() => {
      const s = screen.getAllByRole('combobox').find(el => (el as HTMLSelectElement).value === 'drive:drive-2')
      expect(s).toBeTruthy()
      return s as HTMLSelectElement
    })

    mockFetch.mockClear()
    mockFetch.mockResolvedValue({ ok: true, json: async () => [] })
    fireEvent.change(driveSelect, { target: { value: 'delete' } })

    await waitFor(() => {
      const calls = putCalls()
      expect(calls.length).toBe(1)
      const body = JSON.parse((calls[0][1] as RequestInit).body as string)
      expect(body.action).toBe('delete')
    })
  })

  it('pre-selects the configured drive (value drive:<id>)', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url === '/api/drives') return Promise.resolve({ ok: true, json: async () => [drive1, drive2] })
      if (url === '/api/retention') return Promise.resolve({ ok: true, json: async () => [
        { category: 'with_motion', action: 'send_to_drive', drive_id: 'drive-2' },
      ] })
      return Promise.resolve({ ok: true, json: async () => ({}) })
    })

    render(<MemoryRouter><StorageSettingsPage /></MemoryRouter>)

    await waitFor(() => {
      const s = screen.getAllByRole('combobox').find(el => (el as HTMLSelectElement).value === 'drive:drive-2')
      expect(s).toBeTruthy()
    })
  })
})
