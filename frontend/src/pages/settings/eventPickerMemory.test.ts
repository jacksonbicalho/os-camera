import { afterEach, describe, expect, it } from 'vitest'
import { loadPicked, loadPickerScroll, savePicked, savePickerScroll } from './eventPickerMemory'

afterEach(() => localStorage.clear())

describe('eventPickerMemory', () => {
  it('cai no fallback (scroll 0) quando não há nada salvo', () => {
    expect(loadPickerScroll('cam1')).toBe(0)
  })

  it('round-trip por câmera', () => {
    savePickerScroll('cam1', 240)
    expect(loadPickerScroll('cam1')).toBe(240)
    // outra câmera não compartilha
    expect(loadPickerScroll('cam2')).toBe(0)
  })

  it('arredonda e ignora valores inválidos', () => {
    savePickerScroll('cam1', 240.7)
    expect(loadPickerScroll('cam1')).toBe(241)

    localStorage.setItem('state-event-picker-scroll:cam2', 'abc')
    expect(loadPickerScroll('cam2')).toBe(0)
  })

  it('evento escolhido: fallback null e round-trip (frame + data) por câmera', () => {
    expect(loadPicked('cam1')).toBeNull()

    savePicked('cam1', { frame: '20260610120000_motion.jpg', date: '2026-06-10' })
    expect(loadPicked('cam1')).toEqual({ frame: '20260610120000_motion.jpg', date: '2026-06-10' })
    // outra câmera não compartilha
    expect(loadPicked('cam2')).toBeNull()
  })

  it('evento escolhido: ignora conteúdo corrompido/incompleto', () => {
    localStorage.setItem('state-event-picker-picked:cam1', '{not json')
    expect(loadPicked('cam1')).toBeNull()
    localStorage.setItem('state-event-picker-picked:cam1', JSON.stringify({ frame: 'x' }))
    expect(loadPicked('cam1')).toBeNull()
  })
})
