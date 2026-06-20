import { describe, expect, it } from 'vitest'
import {
  timelineRangeMs,
  timelineWindow,
  timePosFraction,
  isInWindow,
  timelineTicks,
} from './timelineScale'

const HOUR = 3600_000

describe('timelineRangeMs', () => {
  it('durações por range', () => {
    expect(timelineRangeMs('1h')).toBe(HOUR)
    expect(timelineRangeMs('6h')).toBe(6 * HOUR)
    expect(timelineRangeMs('24h')).toBe(24 * HOUR)
    expect(timelineRangeMs('7d')).toBe(7 * 24 * HOUR)
  })
})

describe('timelineWindow', () => {
  it('termina no anchor e recua a duração', () => {
    const end = 10 * HOUR
    expect(timelineWindow(end, '1h')).toEqual({ startMs: 9 * HOUR, endMs: 10 * HOUR })
    expect(timelineWindow(end, '6h')).toEqual({ startMs: 4 * HOUR, endMs: 10 * HOUR })
  })
})

describe('timePosFraction', () => {
  const win = { startMs: 0, endMs: HOUR }
  it('mapeia início/meio/fim', () => {
    expect(timePosFraction(0, win)).toBe(0)
    expect(timePosFraction(HOUR / 2, win)).toBe(0.5)
    expect(timePosFraction(HOUR, win)).toBe(1)
  })
  it('clampa fora da janela', () => {
    expect(timePosFraction(-HOUR, win)).toBe(0)
    expect(timePosFraction(2 * HOUR, win)).toBe(1)
  })
  it('span inválido devolve 0', () => {
    expect(timePosFraction(5, { startMs: 10, endMs: 10 })).toBe(0)
  })
})

describe('isInWindow', () => {
  const win = { startMs: 0, endMs: HOUR }
  it('dentro/fora', () => {
    expect(isInWindow(HOUR / 2, win)).toBe(true)
    expect(isInWindow(0, win)).toBe(true)
    expect(isInWindow(HOUR, win)).toBe(true)
    expect(isInWindow(-1, win)).toBe(false)
    expect(isInWindow(HOUR + 1, win)).toBe(false)
  })
})

describe('timelineTicks', () => {
  it('ticks uniformes inclusive extremos', () => {
    expect(timelineTicks({ startMs: 0, endMs: HOUR }, 3)).toEqual([0, HOUR / 2, HOUR])
  })
  it('count < 2 devolve só o início', () => {
    expect(timelineTicks({ startMs: 5, endMs: HOUR }, 1)).toEqual([5])
  })
})
