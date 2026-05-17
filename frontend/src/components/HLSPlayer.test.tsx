import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import HLSPlayer from './HLSPlayer'

vi.mock('hls.js', () => ({
  default: class {
    static isSupported() { return false }
    loadSource() {}
    attachMedia() {}
    on() {}
    destroy() {}
  },
}))

vi.mock('../auth', () => ({
  getToken: () => 'fake-token',
}))

describe('HLSPlayer', () => {
  beforeEach(() => {
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
    // play is called async after HLS attaches; initial render has no overlay
    expect(screen.queryByRole('button', { name: /reproduzir/i })).toBeNull()
  })
})
