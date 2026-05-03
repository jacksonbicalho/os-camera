import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
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

  it('renders a fullscreen button', () => {
    render(<HLSPlayer src="/stream/cam/index.m3u8" />)
    expect(screen.getByRole('button', { name: /tela inteira/i })).toBeTruthy()
  })

  it('calls requestFullscreen on the video container when fullscreen button is clicked', () => {
    const { container } = render(<HLSPlayer src="/stream/cam/index.m3u8" />)
    const wrapper = container.firstChild as HTMLElement
    wrapper.requestFullscreen = vi.fn().mockResolvedValue(undefined)

    const btn = container.querySelector('button[aria-label="Tela inteira"]') as HTMLElement
    fireEvent.click(btn)

    expect(wrapper.requestFullscreen).toHaveBeenCalled()
  })

  it('calls exitFullscreen when already in fullscreen', () => {
    const { container } = render(<HLSPlayer src="/stream/cam/index.m3u8" />)
    const wrapper = container.firstChild as HTMLElement
    wrapper.requestFullscreen = vi.fn().mockResolvedValue(undefined)
    document.exitFullscreen = vi.fn().mockResolvedValue(undefined)

    Object.defineProperty(document, 'fullscreenElement', { value: wrapper, configurable: true })

    const btn = container.querySelector('button[aria-label="Tela inteira"]') as HTMLElement
    fireEvent.click(btn)

    expect(document.exitFullscreen).toHaveBeenCalled()
    expect(wrapper.requestFullscreen).not.toHaveBeenCalled()
  })

  it('stops click propagation to prevent parent navigation', () => {
    const parentClick = vi.fn()
    const { container } = render(
      <div onClick={parentClick}>
        <HLSPlayer src="/stream/cam/index.m3u8" />
      </div>
    )
    const wrapper = (container.firstChild as HTMLElement).firstChild as HTMLElement
    wrapper.requestFullscreen = vi.fn().mockResolvedValue(undefined)

    const btn = container.querySelector('button[aria-label="Tela inteira"]') as HTMLElement
    fireEvent.click(btn)

    expect(parentClick).not.toHaveBeenCalled()
  })
})
