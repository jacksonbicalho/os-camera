import { afterEach, describe, expect, it } from 'vitest'
import { loadPickerScroll, savePickerScroll } from './eventPickerMemory'

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
})
