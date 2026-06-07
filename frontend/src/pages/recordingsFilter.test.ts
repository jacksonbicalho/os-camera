import { describe, expect, it } from 'vitest'
import { filterRecordings } from './recordingsFilter'

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
