import { describe, expect, it } from 'vitest'
import { zoneThresholdLabel } from './zoneThreshold'

describe('zoneThresholdLabel', () => {
  it('shows live score / zone threshold', () => {
    expect(zoneThresholdLabel(0.013, 0.02, 0.004)).toBe('0.013 / 0.020')
  })

  it('falls back to the global threshold when the zone threshold is 0/undefined', () => {
    expect(zoneThresholdLabel(0.013, 0, 0.004)).toBe('0.013 / 0.004')
    expect(zoneThresholdLabel(0.013, undefined, 0.004)).toBe('0.013 / 0.004')
  })

  it('shows — for the score when there is none yet', () => {
    expect(zoneThresholdLabel(null, 0.02, 0.004)).toBe('— / 0.020')
  })

  it('shows "padrão" when neither threshold is set', () => {
    expect(zoneThresholdLabel(0.5, 0, 0)).toBe('0.500 / padrão')
  })
})
