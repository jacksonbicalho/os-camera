import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
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

describe('retention action — send_to_drive', () => {
  it('pre-selects first drive and sends correct drive_id when switching from delete', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url === '/api/drives') return Promise.resolve({ ok: true, json: async () => [drive1, drive2] })
      if (url === '/api/retention') return Promise.resolve({ ok: true, json: async () => [] })
      return Promise.resolve({ ok: true, json: async () => ({}) })
    })

    render(<MemoryRouter><StorageSettingsPage /></MemoryRouter>)

    await waitFor(() => {
      const selects = screen.getAllByRole('combobox').filter(el => (el as HTMLSelectElement).value === 'delete')
      expect(selects.length).toBeGreaterThan(0)
    })

    mockFetch.mockClear()
    mockFetch.mockResolvedValue({ ok: true, json: async () => [] })

    const actionSelect = screen.getAllByRole('combobox').find(
      el => (el as HTMLSelectElement).value === 'delete'
    )!
    fireEvent.change(actionSelect, { target: { value: 'send_to_drive' } })

    await waitFor(() => {
      const putCalls = mockFetch.mock.calls.filter(
        (c: unknown[]) => (c[0] as string).includes('/api/retention/') && (c[1] as RequestInit)?.method === 'PUT'
      )
      expect(putCalls.length).toBe(1)
      const body = JSON.parse((putCalls[0][1] as RequestInit).body as string)
      expect(body.action).toBe('send_to_drive')
      expect(body.drive_id).toBe('drive-1')
    })
  })

  it('keeps existing drive_id when action was already send_to_drive', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url === '/api/drives') return Promise.resolve({ ok: true, json: async () => [drive1, drive2] })
      if (url === '/api/retention') return Promise.resolve({ ok: true, json: async () => [
        { category: 'with_motion', action: 'send_to_drive', drive_id: 'drive-2' },
      ] })
      return Promise.resolve({ ok: true, json: async () => ({}) })
    })

    render(<MemoryRouter><StorageSettingsPage /></MemoryRouter>)

    // Wait until the drive select (showing drive-2) appears
    await waitFor(() => {
      const driveSelect = screen.getAllByRole('combobox').find(
        el => (el as HTMLSelectElement).value === 'drive-2'
      )
      expect(driveSelect).toBeTruthy()
    })

    // Change drive to drive-1 to verify drive_id is passed correctly
    mockFetch.mockClear()
    mockFetch.mockResolvedValue({ ok: true, json: async () => [] })

    const driveSelect = screen.getAllByRole('combobox').find(
      el => (el as HTMLSelectElement).value === 'drive-2'
    )!
    fireEvent.change(driveSelect, { target: { value: 'drive-1' } })

    await waitFor(() => {
      const putCalls = mockFetch.mock.calls.filter(
        (c: unknown[]) => (c[0] as string).includes('/api/retention/') && (c[1] as RequestInit)?.method === 'PUT'
      )
      expect(putCalls.length).toBe(1)
      const body = JSON.parse((putCalls[0][1] as RequestInit).body as string)
      expect(body.action).toBe('send_to_drive')
      expect(body.drive_id).toBe('drive-1')
    })
  })
})
