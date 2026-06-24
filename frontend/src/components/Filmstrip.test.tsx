import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/react'
import Filmstrip from './Filmstrip'
import type { Recording, MotionEvent } from '../pages/cameraUtils'

function rec(id: number, startMs: number): Recording {
  return { id, filename: `r${id}`, start: new Date(startMs).toISOString(), url: '', is_recording: false, has_motion: false }
}

// O frame com borda é o pai do <img> (o img fica dentro, com overflow-hidden, para o
// zoom de hover não vazar a borda).
function frameOf(thumbId: string): HTMLElement {
  return document.getElementById(thumbId)!.querySelector('img')!.parentElement as HTMLElement
}

afterEach(cleanup)

const baseProps = {
  recordings: [rec(1, 0), rec(2, 300_000)],
  events: [] as MotionEvent[],
  win: { startMs: 0, endMs: 600_000 },
  thumbSrc: (ms: number) => `/thumb?t=${ms}`,
  formatTime: (ms: number) => `${ms}`,
  onSeek: () => {},
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

  it('marca a gravação ativa e pisca a borda só enquanto reproduz (mantém a cor da categoria)', () => {
    const events = [{ id: 9, time: new Date(60_000).toISOString(), score: 0.5, label: 'pessoa' }] as MotionEvent[]
    const { rerender } = render(<Filmstrip {...baseProps} events={events} activeRecordingId={1} playing />)
    const t1 = document.getElementById('filmstrip-1')!
    expect(t1.getAttribute('aria-current')).toBe('true')
    // borda continua na cor da categoria (não vira primary/azul)
    expect(frameOf('filmstrip-1').className).toContain('border-red-500')
    // pisca enquanto reproduz
    expect(frameOf('filmstrip-1').getAttribute('style')).toContain('filmstrip-blink')
    // pausado → não pisca
    rerender(<Filmstrip {...baseProps} events={events} activeRecordingId={1} playing={false} />)
    expect(frameOf('filmstrip-1').getAttribute('style') ?? '').not.toContain('filmstrip-blink')
  })

  it('colore o thumbnail pela categoria do chunk (legenda)', () => {
    // evento "pessoa" dentro do chunk 1 ([0, 5min)); chunk 2 sem evento → contínua
    const events = [{ id: 9, time: new Date(60_000).toISOString(), score: 0.5, label: 'pessoa' }] as MotionEvent[]
    render(<Filmstrip {...baseProps} events={events} />)
    expect(frameOf('filmstrip-1').className).toContain('border-red-500')
    expect(frameOf('filmstrip-2').className).toContain('border-blue-500')
  })

  it('hover usa borda foreground (branca), não primary/azul, e dá zoom na imagem', () => {
    render(<Filmstrip {...baseProps} />)
    const frame = frameOf('filmstrip-1')
    expect(frame.className).toContain('group-hover:border-foreground')
    expect(frame.className).not.toContain('group-hover:border-primary')
    expect(document.getElementById('filmstrip-1')!.querySelector('img')!.className).toContain('group-hover:scale-105')
  })
})
