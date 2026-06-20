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
  dateLabel: '20/06/2026',
  onPrevDate: () => {},
  onNextDate: () => {},
  formatTick: (ms: number) => `${ms}`,
}

describe('HorizontalTimeline', () => {
  it('exibe seletor de janela e navegação de data', () => {
    render(<HorizontalTimeline {...baseProps} />)
    for (const r of ['1h', '6h', '24h', '7d']) {
      expect(document.getElementById(`timeline-range-${r}`), r).toBeTruthy()
    }
    expect(document.getElementById('timeline-date-prev')).toBeTruthy()
    expect(document.getElementById('timeline-date-next')).toBeTruthy()
    expect(document.getElementById('timeline-date-label')!.textContent).toContain('20/06/2026')
  })

  it('clicar num range dispara onRangeChange', () => {
    const onRangeChange = vi.fn()
    render(<HorizontalTimeline {...baseProps} onRangeChange={onRangeChange} />)
    fireEvent.click(document.getElementById('timeline-range-6h')!)
    expect(onRangeChange).toHaveBeenCalledWith('6h')
  })

  it('navegação de data dispara callbacks', () => {
    const onPrevDate = vi.fn()
    const onNextDate = vi.fn()
    render(<HorizontalTimeline {...baseProps} onPrevDate={onPrevDate} onNextDate={onNextDate} />)
    fireEvent.click(document.getElementById('timeline-date-prev')!)
    fireEvent.click(document.getElementById('timeline-date-next')!)
    expect(onPrevDate).toHaveBeenCalled()
    expect(onNextDate).toHaveBeenCalled()
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
})
