import { describe, expect, it } from 'vitest'
import { recordingsForEventWindow } from './eventRecordings'
import type { Recording } from './cameraUtils'

function rec(filename: string, start: string): Recording {
  return { id: 0, filename, start, url: `/rec/${filename}`, is_recording: false, has_motion: true }
}

const recs = [
  rec('a', '2026-06-07T10:00:00Z'),
  rec('b', '2026-06-07T10:05:00Z'),
  rec('c', '2026-06-07T10:10:00Z'),
]

describe('recordingsForEventWindow', () => {
  it('returns the single chunk containing the event', () => {
    const out = recordingsForEventWindow(recs, '2026-06-07T10:06:00Z', 30, 30)
    expect(out.map(r => r.filename)).toEqual(['b'])
  })

  it('returns both chunks when the window crosses a boundary', () => {
    const out = recordingsForEventWindow(recs, '2026-06-07T10:05:05Z', 30, 0)
    expect(out.map(r => r.filename)).toEqual(['a', 'b'])
  })

  it('returns the last/ongoing chunk for a recent event', () => {
    const out = recordingsForEventWindow(recs, '2026-06-07T10:30:00Z', 10, 10)
    expect(out.map(r => r.filename)).toEqual(['c'])
  })

  it('returns nothing when no recording overlaps the window', () => {
    expect(recordingsForEventWindow(recs, '2026-06-07T09:00:00Z', 10, 10)).toEqual([])
  })

  it('returns nothing for an invalid event time', () => {
    expect(recordingsForEventWindow(recs, 'nope', 10, 10)).toEqual([])
  })
})
