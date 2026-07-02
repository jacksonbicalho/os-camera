import { afterEach, describe, it, expect, vi } from 'vitest'
import { cleanup, render, screen, fireEvent } from '@testing-library/react'
import CameraForm from './CameraForm'

afterEach(cleanup)

function renderForm() {
  const onSave = vi.fn().mockResolvedValue(undefined)
  const onCancel = vi.fn()
  render(<CameraForm onSave={onSave} onCancel={onCancel} saving={false} />)
  return { onSave, onCancel }
}

describe('CameraForm advanced section', () => {
  it('hides advanced streaming fields by default and shows the essentials', () => {
    renderForm()
    // Essentials always visible
    expect(screen.getByText('Nome')).toBeTruthy()
    expect(screen.getByText('RTSP URL')).toBeTruthy()
    expect(screen.getByText('Transporte do ao-vivo')).toBeTruthy()
    // Advanced streaming knobs collapsed
    expect(screen.queryByText('Modo de vídeo HLS')).toBeNull()
    expect(screen.queryByText('Retenção DVR (s)')).toBeNull()
    expect(screen.queryByText('Codec de vídeo')).toBeNull()
  })

  it('reveals the advanced fields when the toggle is clicked', () => {
    renderForm()
    const toggle = document.getElementById('camera-advanced-toggle') as HTMLButtonElement
    expect(toggle.getAttribute('aria-expanded')).toBe('false')

    fireEvent.click(toggle)

    expect(toggle.getAttribute('aria-expanded')).toBe('true')
    expect(screen.getByText('Modo de vídeo HLS')).toBeTruthy()
    expect(screen.getByText('Retenção DVR (s)')).toBeTruthy()
  })

  it('submits all fields (advanced included) even while the section stays collapsed', () => {
    const { onSave } = renderForm()
    fireEvent.change(screen.getByPlaceholderText('Sala, Garagem, Entrada'), { target: { value: 'Garagem' } })
    fireEvent.change(screen.getByPlaceholderText('rtsp://usuario:senha@ip:554/stream'), { target: { value: 'rtsp://x/main' } })

    fireEvent.click(screen.getByRole('button', { name: /^salvar$/i }))

    expect(onSave).toHaveBeenCalledTimes(1)
    const data = onSave.mock.calls[0][0]
    expect(data.name).toBe('Garagem')
    expect(data.rtsp_url).toBe('rtsp://x/main')
    // Advanced fields keep their defaults in the submitted form, though never shown.
    expect(data.hls_video_mode).toBe('auto')
    expect(data.record_video_mode).toBe('auto')
  })
})
