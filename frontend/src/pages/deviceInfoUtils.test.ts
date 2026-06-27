import { describe, it, expect } from 'vitest'
import { groupDeviceInfo } from './deviceInfoUtils'

describe('groupDeviceInfo', () => {
  const values: Record<string, string> = {
    collector: 'dahua',
    vendor: 'IntelBras',
    model: 'iM5-SC',
    serial: 'DVB0006008586',
    firmware: '2.800.00IB01X.0.R',
    mac: '54:ba:d9:13:17:97',
    'ntp.enabled': 'true',
    timezone: '22',
    'stream.main.codec': 'H.264',
    'stream.main.gop': '40',
    'stream.sub.width': '640',
    'raw.table.NTP.Enable': 'true',
    'raw.table.Encode[0].MainFormat[0].Video.GOP': '40',
  }

  const grouped = groupDeviceInfo(values)

  function section(title: string) {
    return grouped.sections.find((s) => s.title === title)
  }
  function field(title: string, key: string) {
    return section(title)?.fields.find((f) => f.key === key)
  }

  it('puts identity fields with friendly labels', () => {
    expect(field('Identidade', 'model')).toEqual({ key: 'model', label: 'Modelo', value: 'iM5-SC' })
    expect(field('Identidade', 'serial')?.value).toBe('DVB0006008586')
  })

  it('mostra Conexão (Integrada/USB) na Identidade (webcam)', () => {
    const g = groupDeviceInfo({ collector: 'webcam', model: 'HD Webcam', vendor: 'Sonix', connection: 'Integrada' })
    const id = g.sections.find((s) => s.title === 'Identidade')
    expect(id?.fields.find((f) => f.key === 'connection')).toEqual({ key: 'connection', label: 'Conexão', value: 'Integrada' })
  })

  it('groups main and sub streams separately', () => {
    expect(field('Stream principal', 'stream.main.gop')).toMatchObject({ label: 'GOP', value: '40' })
    expect(field('Stream secundário', 'stream.sub.width')).toMatchObject({ label: 'Largura', value: '640' })
  })

  it('isolates raw.* keys with the prefix stripped, sorted', () => {
    const rawKeys = grouped.raw.map((f) => f.key)
    expect(rawKeys).toContain('table.NTP.Enable')
    expect(rawKeys).toContain('table.Encode[0].MainFormat[0].Video.GOP')
    expect(grouped.raw.every((f) => !f.key.startsWith('raw.'))).toBe(true)
    expect(rawKeys).toEqual([...rawKeys].sort())
  })

  it('omits empty sections', () => {
    const onlyModel = groupDeviceInfo({ model: 'X' })
    expect(onlyModel.sections.find((s) => s.title === 'Rede')).toBeUndefined()
    expect(onlyModel.raw).toEqual([])
  })
})
