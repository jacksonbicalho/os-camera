import { describe, it, expect } from 'vitest'
import { mergeRecordings, eventsWithinRecordings } from './cameraUtils'
import type { Recording, MotionEvent } from './cameraUtils'

function rec(filename: string): Recording {
  const ts = filename.replace('.mp4', '')
  return {
    filename,
    start: `2026-05-06T${ts.slice(8, 10)}:${ts.slice(10, 12)}:${ts.slice(12, 14)}Z`,
    url: `/recordings/${filename}`,
    is_recording: false,
  }
}

function event(timeISO: string, score = 0.05): MotionEvent {
  return { time: timeISO, score }
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
})

// ─── eventsWithinRecordings ─────────────────────────────────────────────────

describe('eventsWithinRecordings', () => {
  it('returns empty when no recordings', () => {
    const ev = event('2026-05-06T10:00:30Z')
    expect(eventsWithinRecordings([ev], [])).toHaveLength(0)
  })

  it('keeps event that falls within a recording range', () => {
    const r = rec('20260506100000.mp4') // starts 10:00:00
    const ev = event('2026-05-06T10:00:30Z')
    expect(eventsWithinRecordings([ev], [r])).toHaveLength(1)
  })

  it('removes event that falls after all recordings (recording deleted)', () => {
    // recording starts at 10:00, next would start at 10:05 (default window)
    // event at 10:10 is outside → filtered out
    const r = rec('20260506100000.mp4')
    const ev = event('2026-05-06T10:10:00Z')
    expect(eventsWithinRecordings([ev], [r])).toHaveLength(0)
  })

  it('keeps event between two consecutive recordings', () => {
    const r1 = rec('20260506100000.mp4') // 10:00
    const r2 = rec('20260506100100.mp4') // 10:01
    const ev = event('2026-05-06T10:00:30Z') // between r1 start and r2 start
    expect(eventsWithinRecordings([ev], [r1, r2])).toHaveLength(1)
  })

  it('removes event before first recording', () => {
    const r = rec('20260506100000.mp4')
    const ev = event('2026-05-06T09:59:00Z')
    expect(eventsWithinRecordings([ev], [r])).toHaveLength(0)
  })
})
