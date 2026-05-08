import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { authHeaders, clearToken } from '../auth'

export interface MotionSettings {
  enabled: boolean
  threshold: number
  fps: number
  cooldown_seconds: number
}

export interface CameraSettings {
  id: string
  rtsp_url: string
  chunk_duration: string
  reconnect_interval: string
  video_codec: string
  has_audio: boolean | null
  width: number
  height: number
  motion: MotionSettings | null
}

export interface Settings {
  timezone: string
  debug: boolean
  log: {
    output: string
    path: string
  }
  server: {
    port: number
    segments_path: string
    recordings_path: string
    hls_dvr_seconds: number
    username: string
  }
  storage: {
    path: string
    retention_minutes: number
    interval_minutes: number
    max_size_gb: number
    warn_percent: number
  }
  motion: MotionSettings
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

export function useSettings(redirectTo: string) {
  const navigate = useNavigate()
  const [settings, setSettings] = useState<Settings | null>(null)

  useEffect(() => {
    fetch('/api/settings', { headers: authHeaders() })
      .then(res => {
        if (res.status === 401) { clearToken(); navigate('/login', { state: { from: redirectTo }, replace: true }); return null }
        return res.json()
      })
      .then(data => { if (data) setSettings(data) })
      .catch(() => {})
  }, [navigate, redirectTo])

  return settings
}

export function useAbout(redirectTo: string) {
  const navigate = useNavigate()
  const [about, setAbout] = useState<AboutInfo | null>(null)

  useEffect(() => {
    fetch('/api/about', { headers: authHeaders() })
      .then(res => {
        if (res.status === 401) { clearToken(); navigate('/login', { state: { from: redirectTo }, replace: true }); return null }
        return res.json()
      })
      .then(data => { if (data) setAbout(data) })
      .catch(() => {})
  }, [navigate, redirectTo])

  return about
}
