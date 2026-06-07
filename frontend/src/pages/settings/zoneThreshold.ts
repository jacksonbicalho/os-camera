// zoneThresholdLabel returns the label shown anchored to the selected motion zone:
// the live region score the zone is producing now, alongside its effective threshold
// (score / threshold). A zone threshold of 0/unset falls back to the camera's global
// threshold, mirroring the backend (a zone with threshold 0 uses cfg.Threshold).
export function zoneThresholdLabel(
  score: number | null | undefined,
  zoneThreshold: number | undefined,
  globalThreshold: number,
): string {
  const eff = zoneThreshold && zoneThreshold > 0 ? zoneThreshold : globalThreshold
  const thr = eff > 0 ? eff.toFixed(3) : 'padrão'
  const sc = score != null ? score.toFixed(3) : '—'
  return `${sc} / ${thr}`
}
