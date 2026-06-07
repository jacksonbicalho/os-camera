import { describe, expect, it } from 'vitest'
import { videoDownloadName } from './videoDownload'

describe('videoDownloadName', () => {
  it('combines a sanitized camera name with the recording start', () => {
    expect(videoDownloadName('Hall', '2026-06-07T18:36:04Z')).toBe('Hall_2026-06-07_18-36-04.mp4')
  })

  it('sanitizes spaces and special chars in the camera name', () => {
    expect(videoDownloadName('Quintal / Fundos', '2026-06-07T18:36:04Z')).toBe('Quintal_Fundos_2026-06-07_18-36-04.mp4')
  })

  it('falls back to "camera" when the name is empty', () => {
    expect(videoDownloadName('', '2026-06-07T18:36:04Z')).toBe('camera_2026-06-07_18-36-04.mp4')
    expect(videoDownloadName(undefined, '2026-06-07T18:36:04Z')).toBe('camera_2026-06-07_18-36-04.mp4')
  })

  it('uses "video" stamp when start is not parseable', () => {
    expect(videoDownloadName('Hall', 'nope')).toBe('Hall_video.mp4')
  })
})
