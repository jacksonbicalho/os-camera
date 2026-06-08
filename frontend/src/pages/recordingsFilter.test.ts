import { describe, expect, it } from 'vitest'
import { filterRecordings, recordingsCount } from './recordingsFilter'

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
