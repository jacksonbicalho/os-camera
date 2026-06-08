// filterRecordings returns only recordings with motion when onlyMotion is on,
// otherwise the full list. Generic over the recording shape (only needs has_motion).
export function filterRecordings<T extends { has_motion?: boolean }>(recs: T[], onlyMotion: boolean): T[] {
  return onlyMotion ? recs.filter(r => !!r.has_motion) : recs
}

// recordingsCount is the number shown next to "Gravações". With the motion filter
// on it counts only the loaded motion recordings; off, it prefers the server total
// (recordings hold the whole day, so the filtered length is accurate).
export function recordingsCount<T extends { has_motion?: boolean }>(recs: T[], total: number, onlyMotion: boolean): number {
  if (onlyMotion) return filterRecordings(recs, true).length
  return total || recs.length
}
