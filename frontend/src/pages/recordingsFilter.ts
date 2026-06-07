// filterRecordings returns only recordings with motion when onlyMotion is on,
// otherwise the full list. Generic over the recording shape (only needs has_motion).
export function filterRecordings<T extends { has_motion?: boolean }>(recs: T[], onlyMotion: boolean): T[] {
  return onlyMotion ? recs.filter(r => !!r.has_motion) : recs
}
