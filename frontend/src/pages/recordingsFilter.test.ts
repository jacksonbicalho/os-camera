import { describe, expect, it } from 'vitest'
import { adjacentRecording, filterRecordings, nextRecording, recordingsCount } from './recordingsFilter'

const recs = [
  { filename: 'a', has_motion: true },
  { filename: 'b', has_motion: false },
  { filename: 'c', has_motion: true },
]

describe('filterRecordings', () => {
  it('returns all recordings when onlyMotion is off', () => {
    expect(filterRecordings(recs, false)).toHaveLength(3)
  })

  it('returns only recordings with motion when onlyMotion is on', () => {
    const out = filterRecordings(recs, true)
    expect(out.map(r => r.filename)).toEqual(['a', 'c'])
  })

  it('treats missing has_motion as no motion', () => {
    expect(filterRecordings([{ filename: 'x' }], true)).toHaveLength(0)
  })
})

describe('recordingsCount', () => {
  it('uses the server total when the motion filter is off', () => {
    expect(recordingsCount(recs, 42, false)).toBe(42)
  })

  it('falls back to the loaded length when total is 0', () => {
    expect(recordingsCount(recs, 0, false)).toBe(3)
  })

  it('counts only motion recordings when the filter is on', () => {
    expect(recordingsCount(recs, 42, true)).toBe(2)
  })
})

describe('nextRecording', () => {
  const seq = [
    { filename: 'a', has_motion: true },
    { filename: 'b', has_motion: false },
    { filename: 'c', has_motion: true },
    { filename: 'd', has_motion: false },
  ]

  it('returns the immediate next recording when the filter is off', () => {
    expect(nextRecording(seq, 'a', false)?.filename).toBe('b')
  })

  it('skips recordings without motion when the filter is on', () => {
    expect(nextRecording(seq, 'a', true)?.filename).toBe('c')
  })

  it('returns null at the end of the (filtered) list', () => {
    expect(nextRecording(seq, 'c', true)).toBeNull()
    expect(nextRecording(seq, 'd', false)).toBeNull()
  })

  it('returns null when the current recording is not found', () => {
    expect(nextRecording(seq, 'zzz', false)).toBeNull()
  })

  it('never advances into the in-progress recording', () => {
    const live = [
      { filename: 'a', has_motion: true },
      { filename: 'b', has_motion: true, is_recording: true },
    ]
    expect(nextRecording(live, 'a', false)).toBeNull()
  })
})

describe('adjacentRecording', () => {
  const seq = [
    { filename: 'a', has_motion: true },
    { filename: 'b', has_motion: false },
    { filename: 'c', has_motion: true },
    { filename: 'd', has_motion: false },
  ]

  it('moves to the next/previous recording in the full list (filter off)', () => {
    // desc list: ArrowUp → newer (idx+1 asc), ArrowDown → older (idx-1 asc)
    expect(adjacentRecording(seq, 'a', 'ArrowUp', 'desc', false)?.filename).toBe('b')
    expect(adjacentRecording(seq, 'b', 'ArrowDown', 'desc', false)?.filename).toBe('a')
  })

  it('skips recordings without motion when the filter is on', () => {
    // filtered = [a, c]; from 'a' ArrowUp (desc) → 'c', skipping 'b'
    expect(adjacentRecording(seq, 'a', 'ArrowUp', 'desc', true)?.filename).toBe('c')
    expect(adjacentRecording(seq, 'c', 'ArrowDown', 'desc', true)?.filename).toBe('a')
  })

  it('honors the visual direction with ascending sort order', () => {
    // asc list: ArrowDown → newer (idx+1)
    expect(adjacentRecording(seq, 'a', 'ArrowDown', 'asc', false)?.filename).toBe('b')
    expect(adjacentRecording(seq, 'b', 'ArrowUp', 'asc', false)?.filename).toBe('a')
  })

  it('returns null past the edges of the (filtered) list', () => {
    expect(adjacentRecording(seq, 'c', 'ArrowUp', 'desc', true)).toBeNull()
    expect(adjacentRecording(seq, 'a', 'ArrowDown', 'desc', true)).toBeNull()
  })

  it('returns null when the current recording is not found', () => {
    expect(adjacentRecording(seq, 'zzz', 'ArrowUp', 'desc', false)).toBeNull()
  })
})
