import type { Recording } from './cameraUtils'

// recordingsForEventWindow returns the recordings whose time range overlaps the
// event window [occurred − lead, occurred + trail]. Recordings expose only `start`,
// so each one's end is taken as the next recording's start (the last/ongoing chunk
// extends to +∞), mirroring how the backend links chunks.
export function recordingsForEventWindow(
  recordings: Recording[],
  eventISO: string,
  leadSec: number,
  trailSec: number,
): Recording[] {
  const ev = Date.parse(eventISO)
  if (Number.isNaN(ev)) return []
  const wStart = ev - leadSec * 1000
  const wEnd = ev + trailSec * 1000

  const sorted = recordings
    .filter(r => !Number.isNaN(Date.parse(r.start)))
    .sort((a, b) => Date.parse(a.start) - Date.parse(b.start))

  const out: Recording[] = []
  for (let i = 0; i < sorted.length; i++) {
    const s = Date.parse(sorted[i].start)
    const e = i + 1 < sorted.length ? Date.parse(sorted[i + 1].start) : Infinity
    if (s <= wEnd && e > wStart) out.push(sorted[i])
  }
  return out
}
