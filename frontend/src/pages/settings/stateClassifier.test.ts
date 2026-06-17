import { describe, expect, it } from 'vitest'
import { bboxToCrop, cropToBbox, validateClassifier, type StateClassifier } from './stateClassifier'

function valid(): StateClassifier {
  return {
    name: 'Portão',
    threshold: 0.8,
    trigger_motion: true,
    trigger_interval_seconds: 0,
    crop_x: 0.1, crop_y: 0.1, crop_w: 0.3, crop_h: 0.3,
    min_consecutive: 3,
    enabled: true,
    classes: ['aberto', 'fechado'],
  }
}

describe('bbox ↔ crop', () => {
  it('round-trips', () => {
    const c = bboxToCrop({ x: 0.2, y: 0.3, w: 0.4, h: 0.1 })
    expect(c).toEqual({ crop_x: 0.2, crop_y: 0.3, crop_w: 0.4, crop_h: 0.1 })
    expect(cropToBbox(c)).toEqual({ x: 0.2, y: 0.3, w: 0.4, h: 0.1 })
  })
})

describe('validateClassifier', () => {
  it('aceita config válida', () => {
    expect(validateClassifier(valid())).toBeNull()
  })
  it('rejeita nome vazio', () => {
    expect(validateClassifier({ ...valid(), name: '  ' })).toBeTruthy()
  })
  it('exige ≥ 2 classes (ignora vazias)', () => {
    expect(validateClassifier({ ...valid(), classes: ['aberto', ' '] })).toBeTruthy()
  })
  it('rejeita crop fora de [0,1]', () => {
    expect(validateClassifier({ ...valid(), crop_x: 0.8, crop_w: 0.5 })).toBeTruthy()
    expect(validateClassifier({ ...valid(), crop_w: 0 })).toBeTruthy()
  })
  it('exige um gatilho', () => {
    expect(validateClassifier({ ...valid(), trigger_motion: false, trigger_interval_seconds: 0 })).toBeTruthy()
  })
})
