import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import { createRef } from 'react'
import HLSPlayer, { type HLSPlayerHandle } from './HLSPlayer'

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

describe('HLSPlayer', () => {
  beforeEach(() => {
    manifestParsedHandler = null
    HTMLVideoElement.prototype.play = vi.fn().mockResolvedValue(undefined)
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
})
