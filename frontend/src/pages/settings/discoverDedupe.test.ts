import { describe, expect, it } from 'vitest'
import { findRegisteredCamera, findRegisteredCameraName, identityLines, discoveredDisplayName } from './discoverDedupe'

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

describe('findRegisteredCamera', () => {
  it('devolve a câmera cadastrada casada pelo IP', () => {
    expect(findRegisteredCamera('192.168.1.10', cameras)?.id).toBe('a')
  })
  it('null quando o IP não está cadastrado ou é vazio', () => {
    expect(findRegisteredCamera('192.168.1.99', cameras)).toBeNull()
    expect(findRegisteredCamera('', cameras)).toBeNull()
  })
})

describe('identityLines', () => {
  it('monta Modelo/Serial/Fabricante/Firmware na ordem, omitindo vazios', () => {
    const lines = identityLines({ model: 'iM5-SC', serial: 'DVB123', vendor: 'Intelbras', firmware: '2.8', hardware: 'x', collector: 'dahua' })
    expect(lines).toEqual([
      { label: 'Modelo', value: 'iM5-SC' },
      { label: 'Serial', value: 'DVB123' },
      { label: 'Fabricante', value: 'Intelbras' },
      { label: 'Firmware', value: '2.8' },
    ])
  })
  it('omite campos ausentes/vazios e ignora hardware/collector', () => {
    expect(identityLines({ model: 'X', firmware: '' })).toEqual([{ label: 'Modelo', value: 'X' }])
    expect(identityLines({})).toEqual([])
  })
})

describe('discoveredDisplayName', () => {
  it('usa o nome do scan quando presente', () => {
    expect(discoveredDisplayName({ ip: '192.168.1.10', name: 'DH-Cam' }, cameras)).toBe('DH-Cam')
  })
  it('sem nome de scan, cai no nome da câmera cadastrada', () => {
    expect(discoveredDisplayName({ ip: '192.168.1.10' }, cameras)).toBe('Corredor')
  })
  it('sem nome e não cadastrada → vazio', () => {
    expect(discoveredDisplayName({ ip: '192.168.1.99' }, cameras)).toBe('')
  })
})
