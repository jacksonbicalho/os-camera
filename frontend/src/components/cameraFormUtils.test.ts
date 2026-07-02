import { describe, it, expect } from 'vitest'
import { emptyForm, formToPayload, type Camera } from './cameraFormUtils'

describe('cameraFormUtils live_transport', () => {
  it('defaults to auto for a new camera', () => {
    expect(emptyForm().live_transport).toBe('auto')
  })

  it('reads the camera value when editing', () => {
    const cam = { id: 'c', name: 'C', rtsp_url: 'rtsp://x', live_transport: 'hls' } as Camera
    expect(emptyForm(cam).live_transport).toBe('hls')
  })

  it('defaults to auto when the camera has no preference', () => {
    const cam = { id: 'c', name: 'C', rtsp_url: 'rtsp://x' } as Camera
    expect(emptyForm(cam).live_transport).toBe('auto')
  })

  it('serializes live_transport into the payload', () => {
    const form = emptyForm()
    form.live_transport = 'webrtc'
    expect(formToPayload(form).live_transport).toBe('webrtc')
  })
})
