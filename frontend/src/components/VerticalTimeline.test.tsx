import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/react'
import VerticalTimeline from './VerticalTimeline'
import type { Recording } from '../pages/cameraUtils'

afterEach(cleanup)

function rec(id: number, start: string): Recording {
  return {
    id,
    filename: start.replace(/[-:TZ]/g, '').slice(0, 14) + '.mp4',
    start,
    url: '',
    is_recording: false,
    has_motion: false,
  }
}

const recordings: Recording[] = [
  rec(1, '2026-06-10T23:00:00Z'),
  rec(2, '2026-06-10T23:05:00Z'),
  rec(3, '2026-06-10T23:10:00Z'),
]

function renderTimeline(onSeek = vi.fn(), activeTime = '2026-06-10T23:10:23Z') {
  return render(
    <VerticalTimeline
      recordings={recordings}
      motionEvents={[]}
      activeRecording={null}
      activeTime={activeTime}
      timezone="UTC"
      onSeek={onSeek}
    />,
  )
}

describe('VerticalTimeline — ponteiro arrastável', () => {
  it('o elemento da timeline tem id "vertical-timeline"', () => {
    const { container } = renderTimeline()
    expect(container.querySelector('#vertical-timeline')).toBeTruthy()
  })

  it('exibe um ponteiro com a hora em HH:MM:SS', () => {
    renderTimeline()
    const pointer = document.getElementById('timeline-pointer')
    expect(pointer).toBeTruthy()
    const time = document.getElementById('timeline-pointer-time')!
    expect(time.textContent).toMatch(/^\d{2}:\d{2}:\d{2}$/)
    expect(time.textContent).toBe('23:10:23')
  })

  it('exibe o ponteiro mesmo sem reprodução ativa (activeTime null)', () => {
    render(
      <VerticalTimeline
        recordings={recordings}
        motionEvents={[]}
        activeRecording={null}
        activeTime={null}
        timezone="UTC"
        onSeek={vi.fn()}
      />,
    )
    expect(document.getElementById('timeline-pointer')).toBeTruthy()
    expect(document.getElementById('timeline-pointer-time')!.textContent).toMatch(/^\d{2}:\d{2}:\d{2}$/)
  })

  it('a régua exibe rótulos em HH:MM:SS, sincronizados com o ponteiro', () => {
    const { container } = renderTimeline()
    const rulerLabels = Array.from(container.querySelectorAll('span'))
      .filter(s => s.id !== 'timeline-pointer-time' && /^\d{2}:\d{2}:\d{2}$/.test(s.textContent ?? ''))
    expect(rulerLabels.length).toBeGreaterThan(0)
    // marca de hora cheia no formato completo
    expect(rulerLabels.some(s => s.textContent === '00:00:00')).toBe(true)
  })

  it('mostra os segundos entre os minutos no zoom alto', () => {
    const desc = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'clientHeight')
    Object.defineProperty(HTMLElement.prototype, 'clientHeight', { configurable: true, get: () => 800 })
    try {
      const { container, getByTitle } = renderTimeline()
      const zoomIn = getByTitle('Aumentar zoom')
      for (let i = 0; i < 6; i++) fireEvent.click(zoomIn) // 1→64×
      const texts = Array.from(container.querySelectorAll('span')).map(s => s.textContent ?? '')
      // rótulos de segundo (segundos != 00) entre os minutos
      expect(texts.some(t => /^\d{2}:\d{2}:(05|10|15|30|45)$/.test(t))).toBe(true)
    } finally {
      if (desc) Object.defineProperty(HTMLElement.prototype, 'clientHeight', desc)
    }
  })

  it('renderiza a gravação ativa por último entre os chunks do mesmo minuto', () => {
    // dois chunks no mesmo minuto (23:00); o ativo é o mais antigo
    const sameMin = [rec(1, '2026-06-10T23:00:00Z'), rec(2, '2026-06-10T23:00:05Z')]
    const active = sameMin[0]
    const { container } = render(
      <VerticalTimeline
        recordings={sameMin}
        motionEvents={[]}
        activeRecording={active}
        activeTime={active.start}
        timezone="UTC"
        onSeek={vi.fn()}
      />,
    )
    const blocks = Array.from(container.querySelectorAll('[data-rec]'))
    expect(blocks.length).toBe(2)
    // o ativo deve ser desenhado por último (fica por cima, bloco azul visível)
    expect(blocks[blocks.length - 1].getAttribute('data-rec')).toBe(active.filename)
  })

  it('a gravação ativa tem altura mínima visível mesmo em zoom baixo', () => {
    const recs = [rec(1, '2026-06-10T23:00:00Z')]
    const { container } = render(
      <VerticalTimeline
        recordings={recs}
        motionEvents={[]}
        activeRecording={recs[0]}
        activeTime={recs[0].start}
        timezone="UTC"
        onSeek={vi.fn()}
      />,
    )
    const block = container.querySelector(`[data-rec="${recs[0].filename}"]`) as HTMLElement
    expect(block).toBeTruthy()
    // sem o mínimo, um chunk de 10s no zoom 1 teria ~3px e sumiria sob o ponteiro
    expect(parseFloat(block.style.height)).toBeGreaterThanOrEqual(20)
  })

  it('arrastar o ponteiro chama onSeek (scrubber)', () => {
    const onSeek = vi.fn()
    const spy = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
      top: 0, left: 0, bottom: 45, right: 72, width: 72, height: 45, x: 0, y: 0,
      toJSON: () => ({}),
    } as DOMRect)

    renderTimeline(onSeek)
    const pointer = document.getElementById('timeline-pointer')!
    fireEvent.mouseDown(pointer)
    // 24h range, px=3, desc: Y=159 → minFloat 1387 ≈ 23:07 (dentro de 23:05).
    fireEvent.mouseMove(window, { clientY: 159 })
    fireEvent.mouseUp(window, { clientY: 159 })

    expect(onSeek).toHaveBeenCalled()
    spy.mockRestore()
  })

  it('soltar num vão sem gravação chama onGap, não onSeek', () => {
    const onSeek = vi.fn()
    const onGap = vi.fn()
    const spy = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
      top: 0, left: 0, bottom: 105, right: 72, width: 72, height: 105, x: 0, y: 0,
      toJSON: () => ({}),
    } as DOMRect)

    // chunk típico = 5min; 23:05→23:30 é um vão (intervalo >> 5min).
    const gapped: Recording[] = [
      rec(1, '2026-06-10T23:00:00Z'),
      rec(2, '2026-06-10T23:05:00Z'),
      rec(3, '2026-06-10T23:30:00Z'),
    ]
    render(
      <VerticalTimeline
        recordings={gapped}
        motionEvents={[]}
        activeRecording={null}
        activeTime={'2026-06-10T23:00:30Z'}
        timezone="UTC"
        onSeek={onSeek}
        onGap={onGap}
      />,
    )
    const pointer = document.getElementById('timeline-pointer')!
    fireEvent.mouseDown(pointer)
    // 24h range, px=3, desc: Y=135 → minFloat 1395 ≈ 23:15 (no vão 23:10–23:30).
    fireEvent.mouseMove(window, { clientY: 135 })
    fireEvent.mouseUp(window, { clientY: 135 })

    expect(onGap).toHaveBeenCalled()
    expect(onSeek).not.toHaveBeenCalled()
    spy.mockRestore()
  })
})
