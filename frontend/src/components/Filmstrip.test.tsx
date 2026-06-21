import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/react'
import Filmstrip from './Filmstrip'
import type { Recording } from '../pages/cameraUtils'

function rec(id: number, startMs: number): Recording {
  return { id, filename: `r${id}`, start: new Date(startMs).toISOString(), url: '', is_recording: false, has_motion: false }
}

afterEach(cleanup)

const baseProps = {
  recordings: [rec(1, 0), rec(2, 300_000)],
  win: { startMs: 0, endMs: 600_000 },
  thumbSrc: (ms: number) => `/thumb?t=${ms}`,
  formatTime: (ms: number) => `${ms}`,
  onSeek: () => {},
  count: 10,
}

describe('Filmstrip', () => {
  it('renderiza uma miniatura por gravação amostrada, com horário e src', () => {
    render(<Filmstrip {...baseProps} />)
    const t1 = document.getElementById('filmstrip-1')!
    expect(t1).toBeTruthy()
    expect(t1.querySelector('img')!.getAttribute('src')).toBe('/thumb?t=0')
    expect(document.getElementById('filmstrip-2')).toBeTruthy()
  })

  it('clicar numa miniatura dispara onSeek(rec, offset)', () => {
    const onSeek = vi.fn()
    render(<Filmstrip {...baseProps} onSeek={onSeek} />)
    fireEvent.click(document.getElementById('filmstrip-2')!)
    expect(onSeek).toHaveBeenCalled()
    expect(onSeek.mock.calls[0][0].id).toBe(2)
  })

  it('janela sem gravações não renderiza nada', () => {
    render(<Filmstrip {...baseProps} recordings={[]} />)
    expect(document.getElementById('filmstrip')).toBeNull()
  })

  it('destaca o thumbnail da gravação ativa', () => {
    render(<Filmstrip {...baseProps} activeRecordingId={2} />)
    expect(document.getElementById('filmstrip-2')!.getAttribute('aria-current')).toBe('true')
    expect(document.getElementById('filmstrip-2')!.querySelector('img')!.className).toContain('ring-primary')
    expect(document.getElementById('filmstrip-1')!.getAttribute('aria-current')).toBeNull()
  })
})
