export interface MotionConfig {
  enabled: boolean
  threshold: number
  fps: number
  cooldown_seconds: number
  capture_width?: number
  capture_height?: number
  playback_lead_seconds?: number
}

export interface Camera {
  name: string
  id: string
  rtsp_url: string
  chunk_duration: string
  reconnect_interval: string
  video_codec: string
  has_audio: boolean | null
  width: number
  height: number
  hls_video_mode: string
  record_video_mode: string
  hls_segment_seconds: number | null
  hls_list_size: number | null
  recording_enabled: boolean
  motion: MotionConfig | null
}

export interface CameraFormData {
  name: string
  rtsp_url: string
  chunk_duration: string
  reconnect_interval: string
  video_codec: string
  has_audio: '' | 'true' | 'false'
  resolution: string
  hls_video_mode: string
  record_video_mode: string
  hls_segment_seconds_default: boolean  // true = usar padrão (null no banco)
  hls_segment_seconds: string
  hls_list_size_default: boolean        // true = usar padrão (null no banco)
  hls_list_size: string
  motion_enabled: boolean
  motion_threshold: string
  motion_fps: string
  motion_cooldown: string
  motion_capture_auto: boolean
  motion_capture_pct: number
  motion_playback_lead: string
}

export const RESOLUTIONS = [
  { label: 'Auto', value: '0x0' },
  { label: '352 × 288 (CIF)', value: '352x288' },
  { label: '640 × 480 (VGA)', value: '640x480' },
  { label: '720 × 576 (D1)', value: '720x576' },
  { label: '1280 × 720 (HD)', value: '1280x720' },
  { label: '1920 × 1080 (Full HD)', value: '1920x1080' },
  { label: '2560 × 1440 (2K)', value: '2560x1440' },
  { label: '3840 × 2160 (4K)', value: '3840x2160' },
]

export function encodeResolution(w: number, h: number): string {
  if (w === 0 || h === 0) return '0x0'
  const match = RESOLUTIONS.find(r => r.value === `${w}x${h}`)
  return match ? match.value : `${w}x${h}`
}

export function decodeResolution(value: string): { width: number; height: number } {
  const [w, h] = value.split('x').map(Number)
  return { width: w || 0, height: h || 0 }
}

function capturePct(capW: number, streamW: number): number {
  if (capW > 0 && streamW > 0) return Math.round(capW / streamW * 100)
  return 25
}

export function emptyForm(cam?: Camera): CameraFormData {
  if (!cam) {
    return {
      name: '',
      rtsp_url: '', chunk_duration: '5m', reconnect_interval: '30s',
      video_codec: '', has_audio: '', resolution: '0x0',
      hls_video_mode: 'auto',
      record_video_mode: 'auto',
      hls_segment_seconds_default: true, hls_segment_seconds: '2',
      hls_list_size_default: true, hls_list_size: '5',
      motion_enabled: false, motion_threshold: '0.02', motion_fps: '2', motion_cooldown: '30',
      motion_capture_auto: true, motion_capture_pct: 25, motion_playback_lead: '10',
    }
  }
  const capW = cam.motion?.capture_width ?? 0
  const auto = capW === 0
  return {
    name: cam.name ?? '',
    rtsp_url: cam.rtsp_url,
    chunk_duration: cam.chunk_duration,
    reconnect_interval: cam.reconnect_interval,
    video_codec: cam.video_codec ?? '',
    has_audio: cam.has_audio == null ? '' : cam.has_audio ? 'true' : 'false',
    resolution: encodeResolution(cam.width ?? 0, cam.height ?? 0),
    hls_video_mode: cam.hls_video_mode || 'auto',
    record_video_mode: cam.record_video_mode || 'auto',
    hls_segment_seconds_default: cam.hls_segment_seconds == null,
    hls_segment_seconds: String(cam.hls_segment_seconds ?? 2),
    hls_list_size_default: cam.hls_list_size == null,
    hls_list_size: String(cam.hls_list_size ?? 5),
    motion_enabled: cam.motion?.enabled ?? false,
    motion_threshold: String(cam.motion?.threshold ?? 0.02),
    motion_fps: String(cam.motion?.fps ?? 2),
    motion_cooldown: String(cam.motion?.cooldown_seconds ?? 30),
    motion_capture_auto: auto,
    motion_capture_pct: capturePct(capW, cam.width ?? 0),
    motion_playback_lead: String(cam.motion?.playback_lead_seconds ?? 10),
  }
}

export function formToPayload(f: CameraFormData) {
  const { width, height } = decodeResolution(f.resolution)
  const payload: Record<string, unknown> = {
    name: f.name,
    rtsp_url: f.rtsp_url,
    chunk_duration: f.chunk_duration || '5m',
    reconnect_interval: f.reconnect_interval || '30s',
    video_codec: f.video_codec,
    has_audio: f.has_audio === '' ? null : f.has_audio === 'true',
    width,
    height,
    hls_video_mode: f.hls_video_mode || 'auto',
    record_video_mode: f.record_video_mode || 'auto',
    hls_segment_seconds: f.hls_segment_seconds_default ? null : (parseInt(f.hls_segment_seconds) || 2),
    hls_list_size: f.hls_list_size_default ? null : (parseInt(f.hls_list_size) || 5),
    motion: {
      enabled: f.motion_enabled,
      threshold: parseFloat(f.motion_threshold) || 0.02,
      fps: parseInt(f.motion_fps) || 2,
      cooldown_seconds: parseInt(f.motion_cooldown) || 30,
      capture_width: f.motion_capture_auto ? 0 : Math.round(width * f.motion_capture_pct / 100),
      capture_height: f.motion_capture_auto ? 0 : Math.round(height * f.motion_capture_pct / 100),
      playback_lead_seconds: parseInt(f.motion_playback_lead) || 10,
    },
  }
  return payload
}
