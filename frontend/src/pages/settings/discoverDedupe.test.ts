import { describe, expect, it } from 'vitest'
import { findRegisteredCameraName } from './discoverDedupe'

const cameras = [
  { id: 'a', name: 'Corredor', rtsp_url: 'rtsp://user:pass@192.168.1.10:554/stream' },
  { id: 'b', name: 'Quintal', rtsp_url: 'rtsp://192.168.1.20:554/' },
  { id: 'c', rtsp_url: 'rtsp://192.168.1.30:554/' },
]

describe('findRegisteredCameraName', () => {
  it('retorna o nome da câmera já cadastrada com aquele IP', () => {
    expect(findRegisteredCameraName('192.168.1.10', cameras)).toBe('Corredor')
  })

  it('cai no id quando a câmera não tem nome', () => {
    expect(findRegisteredCameraName('192.168.1.30', cameras)).toBe('c')
  })

  it('retorna null quando o IP não está cadastrado', () => {
    expect(findRegisteredCameraName('192.168.1.99', cameras)).toBeNull()
  })

  it('retorna null para IP vazio', () => {
    expect(findRegisteredCameraName('', cameras)).toBeNull()
  })
})
