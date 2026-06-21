import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/react'
import EventDetailCard from './EventDetailCard'
import type { MotionEvent } from '../pages/cameraUtils'

const ev: MotionEvent = { id: 1, time: '2026-06-20T18:07:40Z', score: 0.94, frame: 'x.jpg', label: 'pessoa' }

afterEach(cleanup)

const baseProps = {
  event: ev,
  cameraName: 'Corredor de entrada',
  durationSeconds: 18,
  thumbSrc: '/thumb.jpg',
  onPlay: () => {},
  onDownload: () => {},
  onMark: () => {},
}

describe('EventDetailCard', () => {
  it('exibe tipo, confiança, duração, câmera e thumbnail', () => {
    render(<EventDetailCard {...baseProps} />)
    const card = document.getElementById('event-detail-card')!
    expect(card.textContent).toContain('Pessoa detectada')
    expect(card.textContent).toContain('94%')
    expect(card.textContent).toContain('00:18')
    expect(card.textContent).toContain('Corredor de entrada')
    expect(card.querySelector('img')!.getAttribute('src')).toBe('/thumb.jpg')
  })

  it('os botões disparam os callbacks', () => {
    const onPlay = vi.fn()
    const onDownload = vi.fn()
    const onMark = vi.fn()
    render(<EventDetailCard {...baseProps} onPlay={onPlay} onDownload={onDownload} onMark={onMark} />)
    fireEvent.click(document.getElementById('event-detail-play')!)
    fireEvent.click(document.getElementById('event-detail-download')!)
    fireEvent.click(document.getElementById('event-detail-mark')!)
    expect(onPlay).toHaveBeenCalled()
    expect(onDownload).toHaveBeenCalled()
    expect(onMark).toHaveBeenCalled()
  })

  it('sem evento exibe estado vazio', () => {
    render(<EventDetailCard {...baseProps} event={null} />)
    expect(document.getElementById('event-detail-card')!.textContent).toContain('Selecione um evento')
  })
})
