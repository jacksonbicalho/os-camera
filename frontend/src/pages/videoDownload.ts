// videoDownloadName builds a friendly download filename for a recording from the
// camera name and the recording start (ISO). The timestamp is taken verbatim from
// the ISO string (UTC components) so it's deterministic.
export function videoDownloadName(cameraName: string | undefined, startISO: string): string {
  const base =
    (cameraName ?? '').trim().replace(/[^\w.-]+/g, '_').replace(/^_+|_+$/g, '') || 'camera'
  const m = startISO.match(/^(\d{4}-\d{2}-\d{2})T(\d{2}):(\d{2}):(\d{2})/)
  const stamp = m ? `${m[1]}_${m[2]}-${m[3]}-${m[4]}` : 'video'
  return `${base}_${stamp}.mp4`
}
