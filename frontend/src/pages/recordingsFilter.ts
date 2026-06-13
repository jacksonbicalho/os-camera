// filterRecordings returns only recordings with motion when onlyMotion is on,
// otherwise the full list. Generic over the recording shape (only needs has_motion).
export function filterRecordings<T extends { has_motion?: boolean }>(recs: T[], onlyMotion: boolean): T[] {
  return onlyMotion ? recs.filter(r => !!r.has_motion) : recs
}

// nextRecording picks the recording that continuous play should advance to after the
// current clip ends. It honors the motion filter (skips recordings without motion when
// onlyMotion is on) and never advances into the in-progress recording. Returns null when
// there is nothing to play next or the current recording isn't in the (filtered) list.
export function nextRecording<T extends { filename: string; is_recording?: boolean; has_motion?: boolean }>(
  recordings: T[],
  currentFilename: string,
  onlyMotion: boolean,
): T | null {
  const asc = [...filterRecordings(recordings, onlyMotion)].sort((a, b) => a.filename.localeCompare(b.filename))
  const idx = asc.findIndex(r => r.filename === currentFilename)
  if (idx === -1) return null
  const next = asc[idx + 1]
  return next && !next.is_recording ? next : null
}

// adjacentRecording picks the recording the Ctrl+arrow shortcut should jump to. It honors
// the motion filter (skips recordings without motion when onlyMotion is on) and maps the
// arrow direction to the visual list order: in a desc list ArrowUp moves to a newer
// recording, ArrowDown to an older one; an asc list is mirrored. Returns null past the
// edges or when the current recording isn't in the (filtered) list.
export function adjacentRecording<T extends { filename: string; has_motion?: boolean }>(
  recordings: T[],
  currentFilename: string,
  key: 'ArrowUp' | 'ArrowDown',
  sortOrder: 'asc' | 'desc',
  onlyMotion: boolean,
): T | null {
  const sorted = [...filterRecordings(recordings, onlyMotion)].sort((a, b) => a.filename.localeCompare(b.filename))
  const idx = sorted.findIndex(r => r.filename === currentFilename)
  if (idx === -1) return null
  const step = sortOrder === 'asc' ? 1 : -1
  const nextIdx = key === 'ArrowDown' ? idx + step : idx - step
  return sorted[nextIdx] ?? null
}

// recordingsCount is the number shown next to "Gravações". With the motion filter
// on it counts only the loaded motion recordings; off, it prefers the server total
// (recordings hold the whole day, so the filtered length is accurate).
export function recordingsCount<T extends { has_motion?: boolean }>(recs: T[], total: number, onlyMotion: boolean): number {
  if (onlyMotion) return filterRecordings(recs, true).length
  return total || recs.length
}
