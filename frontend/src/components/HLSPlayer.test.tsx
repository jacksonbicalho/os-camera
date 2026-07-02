import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import { createRef } from 'react'
import HLSPlayer, { type HLSPlayerHandle } from './HLSPlayer'
import { negotiateWebRTC, WebRTCUnavailableError } from '../lib/webrtc'

type HlsEventHandler = () => void

let manifestParsedHandler: HlsEventHandler | null = null

vi.mock('hls.js', () => ({
  default: class {
    static isSupported() { return true }
    static Events = { MANIFEST_PARSED: 'manifestParsed', ERROR: 'error' }
    loadSource() {}
    attachMedia() {}
    on(event: string, handler: HlsEventHandler) {
      if (event === 'manifestParsed') manifestParsedHandler = handler
    }
    destroy() {}
  },
}))

vi.mock('../auth', () => ({
  getToken: () => 'fake-token',
}))

vi.mock('../lib/webrtc', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/webrtc')>()
  return { ...actual, negotiateWebRTC: vi.fn() }
})

let lastPC: FakeRTCPeerConnection | null = null

class FakeRTCPeerConnection {
  connectionState: RTCPeerConnectionState = 'new'
  ontrack: ((ev: RTCTrackEvent) => void) | null = null
  onconnectionstatechange: (() => void) | null = null
  addTransceiver = vi.fn()
  close = vi.fn()
  fireConnected() {
    this.connectionState = 'connected'
    this.onconnectionstatechange?.()
  }
}

class FakeEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null
  onerror: (() => void) | null = null
  close = vi.fn()
}

// stubs the browser live-view globals: RTCPeerConnection (WebRTC) and
// EventSource (motion SSE, opened by useEventSource when cameraId is set).
function stubRTCPeerConnection() {
  vi.stubGlobal('RTCPeerConnection', function RTCPeerConnectionStub() {
    lastPC = new FakeRTCPeerConnection()
    return lastPC
  })
  vi.stubGlobal('EventSource', FakeEventSource)
}

async function flushAsync() {
  await act(async () => { await new Promise((r) => setTimeout(r, 0)) })
}

describe('HLSPlayer', () => {
  beforeEach(() => {
    manifestParsedHandler = null
    lastPC = null
    ;(negotiateWebRTC as unknown as Mock).mockReset()
    HTMLVideoElement.prototype.play = vi.fn().mockResolvedValue(undefined)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('renders a video element', () => {
    const { container } = render(<HLSPlayer src="/stream/cam/index.m3u8" />)
    expect(container.querySelector('video')).toBeTruthy()
  })

  it('does not render a fullscreen button (fullscreen is in the page header)', () => {
    render(<HLSPlayer src="/stream/cam/index.m3u8" />)
    expect(screen.queryByRole('button', { name: /tela inteira/i })).toBeNull()
  })

  it('renders the play button when playback is blocked', async () => {
    HTMLVideoElement.prototype.play = vi.fn().mockRejectedValue(new Error('blocked'))
    render(<HLSPlayer src="/stream/cam/index.m3u8" />)
    expect(screen.queryByRole('button', { name: /reproduzir/i })).toBeNull()
  })

  it('exposes getVideoElement via handle', () => {
    const ref = createRef<HLSPlayerHandle>()
    const { container } = render(<HLSPlayer src="/stream/cam/index.m3u8" ref={ref} />)
    const video = container.querySelector('video')
    expect(ref.current?.getVideoElement()).toBe(video)
  })

  it('respects muted=false prop after MANIFEST_PARSED — not hardcoded to muted', async () => {
    const { container } = render(<HLSPlayer src="/stream/cam/index.m3u8" muted={false} />)
    const video = container.querySelector('video') as HTMLVideoElement

    // Flush the dynamic import microtask so HLS sets up the event handler
    await act(async () => { await Promise.resolve() })
    expect(manifestParsedHandler).not.toBeNull()

    await act(async () => { manifestParsedHandler!() })

    expect(video.muted).toBe(false)
  })

  it('respects muted=true prop after MANIFEST_PARSED', async () => {
    const { container } = render(<HLSPlayer src="/stream/cam/index.m3u8" muted={true} />)
    const video = container.querySelector('video') as HTMLVideoElement

    await act(async () => { await Promise.resolve() })
    expect(manifestParsedHandler).not.toBeNull()

    await act(async () => { manifestParsedHandler!() })

    expect(video.muted).toBe(true)
  })

  it('falls back to HLS when WebRTC is unavailable (409)', async () => {
    stubRTCPeerConnection()
    ;(negotiateWebRTC as unknown as Mock).mockRejectedValue(new WebRTCUnavailableError())

    render(<HLSPlayer src="/stream/cam/index.m3u8" cameraId="cam" />)
    await flushAsync()

    expect(negotiateWebRTC).toHaveBeenCalled()
    expect(manifestParsedHandler).not.toBeNull() // HLS took over
  })

  it('stays on WebRTC (no HLS setup) when negotiation succeeds', async () => {
    stubRTCPeerConnection()
    ;(negotiateWebRTC as unknown as Mock).mockResolvedValue(undefined)

    const { container } = render(<HLSPlayer src="/stream/cam/index.m3u8" cameraId="cam" />)
    await flushAsync()

    expect(negotiateWebRTC).toHaveBeenCalled()
    expect(manifestParsedHandler).toBeNull() // HLS never set up

    const video = container.querySelector('video') as HTMLVideoElement
    await act(async () => { lastPC!.fireConnected() })
    expect(video.play).toHaveBeenCalled()
  })

  it('does not attempt WebRTC when transport is "hls"', async () => {
    stubRTCPeerConnection()
    ;(negotiateWebRTC as unknown as Mock).mockResolvedValue(undefined)

    render(<HLSPlayer src="/stream/cam/index.m3u8" cameraId="cam" transport="hls" />)
    await flushAsync()

    expect(negotiateWebRTC).not.toHaveBeenCalled()
    expect(manifestParsedHandler).not.toBeNull() // went straight to HLS
  })
})
