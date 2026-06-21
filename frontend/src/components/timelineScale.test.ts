import { describe, expect, it } from 'vitest'
import {
  timelineRangeMs,
  timelineWindow,
  timePosFraction,
  isInWindow,
  timelineTicks,
  posToTime,
  recordingAtMs,
} from './timelineScale'
import type { Recording } from '../pages/cameraUtils'

const HOUR = 3600_000

function rec(id: number, startMs: number, isRecording = false): Recording {
  return { id, filename: `r${id}`, start: new Date(startMs).toISOString(), url: '', is_recording: isRecording, has_motion: false }
}

describe('timelineRangeMs', () => {
  it('durações por range', () => {
    expect(timelineRangeMs('1h')).toBe(HOUR)
    expect(timelineRangeMs('6h')).toBe(6 * HOUR)
    expect(timelineRangeMs('24h')).toBe(24 * HOUR)
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

describe('posToTime', () => {
  const win = { startMs: 0, endMs: HOUR }
  it('inverso de timePosFraction', () => {
    expect(posToTime(0, win)).toBe(0)
    expect(posToTime(0.5, win)).toBe(HOUR / 2)
    expect(posToTime(1, win)).toBe(HOUR)
  })
  it('clampa fração fora de 0..1', () => {
    expect(posToTime(-1, win)).toBe(0)
    expect(posToTime(2, win)).toBe(HOUR)
  })
  it('roundtrip com timePosFraction', () => {
    const win2 = { startMs: 1000, endMs: 1000 + HOUR }
    const t = 1000 + HOUR / 3
    expect(posToTime(timePosFraction(t, win2), win2)).toBeCloseTo(t, 3)
  })
})

describe('recordingAtMs', () => {
  const recs = [rec(1, 0), rec(2, 300_000), rec(3, 600_000, true)]
  const chunk = 300_000
  it('acha a gravação e o offset', () => {
    const r = recordingAtMs(recs, 60_000, chunk)
    expect(r?.rec.id).toBe(1)
    expect(r?.offsetSeconds).toBe(60)
  })
  it('lacuna devolve null', () => {
    expect(recordingAtMs(recs, 1_000_000, chunk)).toBeNull()
  })
  it('ignora gravação ativa (is_recording)', () => {
    expect(recordingAtMs(recs, 650_000, chunk)).toBeNull()
  })
})
