import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import DeviceInfoPanel from './DeviceInfoPanel'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

function okGet(values: Record<string, string>) {
  return {
    ok: true,
    status: 200,
    json: async () => ({ collected_at: '2026-06-12T13:11:09Z', values }),
  }
}

describe('DeviceInfoPanel', () => {
  it('renders captured device info with friendly labels', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(okGet({ model: 'iM5-SC', 'stream.main.gop': '40', 'raw.x': 'y' })))
    render(<DeviceInfoPanel cameraId="cam1" isAdmin={false} />)

    expect(await screen.findByText('iM5-SC')).toBeTruthy()
    expect(screen.getByText('Modelo')).toBeTruthy()
    expect(screen.getByText('GOP')).toBeTruthy()
  })

  it('shows a not-captured message on 404', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 404, text: async () => 'no device info captured' }))
    render(<DeviceInfoPanel cameraId="cam1" isAdmin={true} />)

    expect(await screen.findByText(/ainda não capturado/i)).toBeTruthy()
  })

  it('refreshes via POST when the admin clicks the button', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce({ ok: false, status: 404, text: async () => '' }) // initial GET
      .mockResolvedValueOnce(okGet({ model: 'iM5-SC' })) // POST refresh
    vi.stubGlobal('fetch', fetchMock)
    render(<DeviceInfoPanel cameraId="cam1" isAdmin={true} />)

    const btn = await screen.findByRole('button', { name: /reanalisar|capturar/i })
    fireEvent.click(btn)

    expect(await screen.findByText('iM5-SC')).toBeTruthy()
    const secondCall = fetchMock.mock.calls[1]
    expect(String(secondCall[0])).toContain('/device-info/refresh')
    expect(secondCall[1]?.method).toBe('POST')
  })
})
