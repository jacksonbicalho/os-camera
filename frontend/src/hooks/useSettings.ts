import { useEffect, useState } from 'react'
import { authHeaders, onUnauthorized } from '../auth'

export interface MotionSettings {
  enabled: boolean
  threshold: number
  fps: number
  cooldown_seconds: number
  capture_width?: number
  capture_height?: number
  playback_lead_seconds: number
  playback_trail_seconds: number
}

export interface CameraSettings {
  id: string
  name: string
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
  motion: MotionSettings | null
}

export interface Settings {
  timezone: string
  debug: boolean
  log: {
    output: string
    path: string
    max_size_mb: number
    max_age_days: number
    max_backups: number
    compress: boolean
  }
  server: {
    port: number
    segments_path: string
    recordings_path: string
    username: string
  }
  storage: {
    path: string
    with_motion_minutes: number
    without_motion_minutes: number
    interval_minutes: number
    max_size_gb: number
    warn_percent: number
  }
  defaults: {
    chunk_duration: string
    reconnect_interval: string
  }
  cameras: CameraSettings[]
}

export interface AboutInfo {
  version: string
  commit: string
  built_at: string
  uptime_seconds: number
  go_version: string
}

export function useSettings() {
  const [settings, setSettings] = useState<Settings | null>(null)
  const [key, setKey] = useState(0)

  useEffect(() => {
    fetch('/api/settings', { headers: authHeaders() })
      .then(res => {
        if (res.status === 401) { onUnauthorized(); return null }
        return res.json()
      })
      .then(data => { if (data) setSettings(data) })
      .catch(() => {})
  }, [key])

  const reload = () => setKey(k => k + 1)

  return { settings, reload }
}

export function useAbout() {
  const [about, setAbout] = useState<AboutInfo | null>(null)

  useEffect(() => {
    fetch('/api/about', { headers: authHeaders() })
      .then(res => {
        if (res.status === 401) { onUnauthorized(); return null }
        return res.json()
      })
      .then(data => { if (data) setAbout(data) })
      .catch(() => {})
  }, [])

  return about
}
