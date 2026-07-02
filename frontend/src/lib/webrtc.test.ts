import { describe, it, expect, vi } from 'vitest'
import { negotiateWebRTC, WebRTCUnavailableError } from './webrtc'

function fakePC(overrides: Partial<RTCPeerConnection> = {}): RTCPeerConnection {
  return {
    iceGatheringState: 'complete',
    localDescription: { sdp: 'offer-sdp' },
    createOffer: vi.fn().mockResolvedValue({ type: 'offer', sdp: 'offer-sdp' }),
    setLocalDescription: vi.fn().mockResolvedValue(undefined),
    setRemoteDescription: vi.fn().mockResolvedValue(undefined),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    ...overrides,
  } as unknown as RTCPeerConnection
}

function okResp(sdp: string) {
  return { ok: true, status: 200, json: async () => ({ sdp }) } as unknown as Response
}

describe('negotiateWebRTC', () => {
  it('posts the offer and applies the answer on 200', async () => {
    const pc = fakePC()
    const fetchFn = vi.fn().mockResolvedValue(okResp('answer-sdp'))

    await negotiateWebRTC('cam1', pc, { token: 't', fetchFn })

    expect(fetchFn).toHaveBeenCalledWith(
      '/api/cameras/cam1/webrtc',
      expect.objectContaining({ method: 'POST' }),
    )
    expect(pc.setRemoteDescription).toHaveBeenCalledWith({ type: 'answer', sdp: 'answer-sdp' })
  })

  it('throws WebRTCUnavailableError on 409 (fallback signal)', async () => {
    const pc = fakePC()
    const fetchFn = vi.fn().mockResolvedValue({ ok: false, status: 409, json: async () => ({}) } as unknown as Response)

    await expect(negotiateWebRTC('cam1', pc, { token: 't', fetchFn })).rejects.toBeInstanceOf(
      WebRTCUnavailableError,
    )
    expect(pc.setRemoteDescription).not.toHaveBeenCalled()
  })

  it('throws on other non-ok responses', async () => {
    const pc = fakePC()
    const fetchFn = vi.fn().mockResolvedValue({ ok: false, status: 500, json: async () => ({}) } as unknown as Response)

    await expect(negotiateWebRTC('cam1', pc, { token: 't', fetchFn })).rejects.toThrow()
    expect(pc.setRemoteDescription).not.toHaveBeenCalled()
  })

  it('propagates network errors', async () => {
    const pc = fakePC()
    const fetchFn = vi.fn().mockRejectedValue(new Error('network down'))

    await expect(negotiateWebRTC('cam1', pc, { token: 't', fetchFn })).rejects.toThrow('network down')
  })

  it('sends the auth token in the Authorization header', async () => {
    const pc = fakePC()
    const fetchFn = vi.fn().mockResolvedValue(okResp('a'))

    await negotiateWebRTC('cam1', pc, { token: 'my-token', fetchFn })

    const init = fetchFn.mock.calls[0][1] as RequestInit
    const headers = init.headers as Record<string, string>
    expect(headers.Authorization).toBe('Bearer my-token')
    expect(init.body).toBe(JSON.stringify({ sdp: 'offer-sdp' }))
  })
})
