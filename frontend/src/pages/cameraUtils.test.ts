import { describe, it, expect } from 'vitest'
import { mergeRecordings, parseDurationToMs, secondStepTarget } from './cameraUtils'
import type { Recording } from './cameraUtils'

function rec(filename: string): Recording {
  const ts = filename.replace('.mp4', '')
  return {
    filename,
    start: `2026-05-06T${ts.slice(8, 10)}:${ts.slice(10, 12)}:${ts.slice(12, 14)}Z`,
    url: `/recordings/${filename}`,
    is_recording: false,
  }
}

// ─── mergeRecordings ────────────────────────────────────────────────────────

describe('mergeRecordings', () => {
  it('returns fresh when hasMore is false (complete list)', () => {
    const prev = [rec('20260506120000.mp4'), rec('20260506110000.mp4')]
    const fresh = [rec('20260506120000.mp4')]
    expect(mergeRecordings(prev, fresh, 'desc', false)).toEqual(fresh)
  })

  it('removes recording deleted from page-1 range in desc order', () => {
    const r120 = rec('20260506120000.mp4')
    const r110 = rec('20260506110000.mp4')
    const r100 = rec('20260506100000.mp4')
    // prev has 3; fresh page 1 has r120 and r100 (r110 was deleted)
    // boundary in desc = r100 (oldest fresh); r110 is within that range → removed
    const result = mergeRecordings([r120, r110, r100], [r120, r100], 'desc', true)
    expect(result.map(r => r.filename)).not.toContain('20260506110000.mp4')
  })

  it('keeps beyond-page-1 recording in desc order (older than oldest fresh)', () => {
    const r120 = rec('20260506120000.mp4')
    const r110 = rec('20260506110000.mp4')
    const r090 = rec('20260506090000.mp4') // loaded via "load more", older than r110
    // fresh page 1 = [r120, r110]; r090 is beyond page 1
    const result = mergeRecordings([r120, r110, r090], [r120, r110], 'desc', true)
    expect(result.map(r => r.filename)).toContain('20260506090000.mp4')
  })

  it('removes recording deleted from page-1 range in asc order', () => {
    const r100 = rec('20260506100000.mp4')
    const r110 = rec('20260506110000.mp4')
    const r120 = rec('20260506120000.mp4')
    // prev has 3; fresh page 1 has r100 and r120 (r110 deleted)
    // boundary in asc = r120 (newest fresh); r110 is within that range → removed
    const result = mergeRecordings([r100, r110, r120], [r100, r120], 'asc', true)
    expect(result.map(r => r.filename)).not.toContain('20260506110000.mp4')
  })

  it('adds new recording not previously in prev', () => {
    const r120 = rec('20260506120000.mp4')
    const r110 = rec('20260506110000.mp4')
    const result = mergeRecordings([r120], [r120, r110], 'desc', false)
    expect(result.map(r => r.filename)).toContain('20260506110000.mp4')
  })

  it('updates is_recording flag from fresh result', () => {
    const old = { ...rec('20260506120000.mp4'), is_recording: true }
    const fresh = { ...rec('20260506120000.mp4'), is_recording: false }
    const result = mergeRecordings([old], [fresh], 'desc', false)
    expect(result[0].is_recording).toBe(false)
  })

  it('returns same prev reference when nothing changed', () => {
    const prev = [rec('20260506120000.mp4'), rec('20260506110000.mp4')]
    const fresh = [{ ...prev[0] }, { ...prev[1] }]
    const result = mergeRecordings(prev, fresh, 'desc', false)
    expect(result).toBe(prev)
  })

  it('returns new array when is_recording flag changes', () => {
    const prev = [{ ...rec('20260506120000.mp4'), is_recording: true }]
    const fresh = [{ ...rec('20260506120000.mp4'), is_recording: false }]
    const result = mergeRecordings(prev, fresh, 'desc', false)
    expect(result).not.toBe(prev)
    expect(result[0].is_recording).toBe(false)
  })
})

// ─── parseDurationToMs ──────────────────────────────────────────────────────

describe('parseDurationToMs', () => {
  it('"5m" → 300000', () => expect(parseDurationToMs('5m')).toBe(300_000))
  it('"30s" → 30000', () => expect(parseDurationToMs('30s')).toBe(30_000))
  it('"6s" → 6000', () => expect(parseDurationToMs('6s')).toBe(6_000))
  it('"2h" → 7200000', () => expect(parseDurationToMs('2h')).toBe(7_200_000))
  it('undefined → default 30000', () => expect(parseDurationToMs(undefined)).toBe(30_000))
  it('empty string → default 30000', () => expect(parseDurationToMs('')).toBe(30_000))
})

// ─── secondStepTarget (Ctrl+Shift+seta) ─────────────────────────────────────

describe('secondStepTarget', () => {
  const recs = [
    rec('20260506120000.mp4'), // 12:00:00
    rec('20260506120500.mp4'), // 12:05:00
    rec('20260506121000.mp4'), // 12:10:00
  ]

  it('avança 1s dentro do mesmo chunk', () => {
    expect(secondStepTarget(recs, '20260506120500.mp4', 5, 300, 1))
      .toEqual({ kind: 'same', time: 6 })
  })

  it('retrocede 1s dentro do mesmo chunk', () => {
    expect(secondStepTarget(recs, '20260506120500.mp4', 5, 300, -1))
      .toEqual({ kind: 'same', time: 4 })
  })

  it('avança além do fim → carrega o próximo chunk no overflow', () => {
    const r = secondStepTarget(recs, '20260506120500.mp4', 300, 300, 1)
    expect(r).toMatchObject({ kind: 'load', offsetSeconds: 1 })
    expect(r && r.kind === 'load' && r.rec.filename).toBe('20260506121000.mp4')
  })

  it('retrocede antes do início → carrega o chunk anterior perto do fim', () => {
    const r = secondStepTarget(recs, '20260506120500.mp4', 0, 300, -1)
    expect(r).toMatchObject({ kind: 'load', offsetSeconds: 1, fromEnd: true })
    expect(r && r.kind === 'load' && r.rec.filename).toBe('20260506120000.mp4')
  })

  it('avança no último chunk sem próximo → null', () => {
    expect(secondStepTarget(recs, '20260506121000.mp4', 300, 300, 1)).toBeNull()
  })

  it('retrocede no primeiro chunk sem anterior → null', () => {
    expect(secondStepTarget(recs, '20260506120000.mp4', 0, 300, -1)).toBeNull()
  })
})
