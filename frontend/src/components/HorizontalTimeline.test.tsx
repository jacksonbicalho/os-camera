import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/react'
import HorizontalTimeline from './HorizontalTimeline'
import type { Recording, MotionEvent } from '../pages/cameraUtils'

const HOUR = 3600_000

function ev(id: number, ms: number, label = ''): MotionEvent {
  return { id, time: new Date(ms).toISOString(), score: 0.5, label }
}

afterEach(cleanup)

const baseProps = {
  recordings: [] as Recording[],
  events: [] as MotionEvent[],
  range: '1h' as const,
  onRangeChange: () => {},
  endMs: HOUR, // janela [0, HOUR]
  selectedDate: new Date(0),
  onSelectDate: () => {},
  formatTick: (ms: number) => `${ms}`,
}

describe('HorizontalTimeline', () => {
  it('exibe seletor de janela (sem 7d) e o botão de data', () => {
    render(<HorizontalTimeline {...baseProps} />)
    for (const r of ['1h', '6h', '24h']) {
      expect(document.getElementById(`timeline-range-${r}`), r).toBeTruthy()
    }
    expect(document.getElementById('timeline-range-7d')).toBeNull()
    expect(document.getElementById('timeline-date')!.textContent).toContain('1970')
  })

  it('clicar num range dispara onRangeChange', () => {
    const onRangeChange = vi.fn()
    render(<HorizontalTimeline {...baseProps} onRangeChange={onRangeChange} />)
    fireEvent.click(document.getElementById('timeline-range-6h')!)
    expect(onRangeChange).toHaveBeenCalledWith('6h')
  })

  it('clicar na data abre o calendário em popover', () => {
    render(<HorizontalTimeline {...baseProps} />)
    expect(document.getElementById('timeline-date-popover')).toBeNull()
    fireEvent.click(document.getElementById('timeline-date')!)
    expect(document.getElementById('timeline-date-popover')).toBeTruthy()
  })

  it('posiciona marca de evento no meio da janela e omite fora', () => {
    render(<HorizontalTimeline {...baseProps} events={[ev(1, HOUR / 2, 'pessoa'), ev(2, 2 * HOUR)]} />)
    const mark = document.getElementById('timeline-mark-1')!
    expect(mark).toBeTruthy()
    expect(mark.style.left).toBe('50%')
    expect(document.getElementById('timeline-mark-2')).toBeNull()
  })

  it('marca usa cor por categoria', () => {
    render(<HorizontalTimeline {...baseProps} events={[ev(1, HOUR / 2, 'pessoa')]} />)
    expect(document.getElementById('timeline-mark-1')!.className).toContain('bg-red-500')
  })

  it('renderiza a faixa de gravação contínua na janela', () => {
    const rec: Recording = { id: 1, filename: 'a.mp4', start: new Date(0).toISOString(), url: '', is_recording: false, has_motion: false }
    render(<HorizontalTimeline {...baseProps} recordings={[rec]} />)
    expect(document.getElementById('timeline-rec-1')).toBeTruthy()
  })

  it('legenda lista as categorias', () => {
    render(<HorizontalTimeline {...baseProps} />)
    const legend = document.getElementById('timeline-legend')!.textContent ?? ''
    for (const l of ['Contínua', 'Movimento', 'Pessoa', 'IA', 'Estados']) {
      expect(legend, l).toContain(l)
    }
  })

  it('desenha o ponteiro na posição de reprodução e o omite fora da janela', () => {
    const { rerender } = render(<HorizontalTimeline {...baseProps} playheadMs={HOUR / 2} />)
    const p = document.getElementById('timeline-pointer')!
    expect(p).toBeTruthy()
    expect(p.style.left).toBe('50%')
    rerender(<HorizontalTimeline {...baseProps} playheadMs={2 * HOUR} />)
    expect(document.getElementById('timeline-pointer')).toBeNull()
  })

  it('clique sobre gravação faz onSeek(rec, offset); lacuna faz onGap', () => {
    const onSeek = vi.fn()
    const onGap = vi.fn()
    const rec: Recording = { id: 7, filename: 'a.mp4', start: new Date(0).toISOString(), url: '', is_recording: false, has_motion: false }
    render(<HorizontalTimeline {...baseProps} recordings={[rec]} onSeek={onSeek} onGap={onGap} />)
    const track = document.getElementById('timeline-track')!
    track.getBoundingClientRect = () => ({ left: 0, width: 1000, top: 0, right: 1000, bottom: 0, height: 0, x: 0, y: 0, toJSON() {} }) as DOMRect

    // clientX=0 → fração 0 → ms 0 → dentro do chunk da gravação 7
    fireEvent.click(track, { clientX: 0 })
    expect(onSeek).toHaveBeenCalled()
    expect(onSeek.mock.calls[0][0].id).toBe(7)
    expect(onGap).not.toHaveBeenCalled()

    // clientX=500 → ms = HOUR/2 → lacuna (chunk de 5min só cobre o início)
    fireEvent.click(track, { clientX: 500 })
    expect(onGap).toHaveBeenCalled()
  })
})
