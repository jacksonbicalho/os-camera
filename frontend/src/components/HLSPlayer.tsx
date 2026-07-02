import { forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState } from 'react'
import type HlsType from 'hls.js'
import { getToken } from '../auth'
import { negotiateWebRTC } from '../lib/webrtc'
import { useEventSource } from '../hooks/useEventSource'
import { AlertTriangle, Loader2, Play } from './Icons'

export interface HLSStats {
  bandwidthKbps: number
  latencySeconds: number
}

export interface HLSPlayerHandle {
  seekTo: (isoTime: string) => void
  getStats: () => HLSStats | null
  getVideoQuality: () => VideoPlaybackQuality | null
  getVideoElement: () => HTMLVideoElement | null
}

interface HLSPlayerProps {
  src: string
  className?: string
  containerClassName?: string
  cameraId?: string
  transport?: string
  muted?: boolean
  segmentSeconds?: number | null
  onGoToEvent?: (eventTime: string) => void
}

interface MotionAlert {
  score: number
  time: string
}

const HLSPlayer = forwardRef<HLSPlayerHandle, HLSPlayerProps>(function HLSPlayer({ src, className, containerClassName, cameraId, transport, muted = true, segmentSeconds, onGoToEvent }, ref) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const hlsRef = useRef<HlsType | null>(null)
  const mutedRef = useRef(muted)
  const [motionAlert, setMotionAlert] = useState<MotionAlert | null>(null)
  const alertTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [playBlocked, setPlayBlocked] = useState(false)
  const [fatalError, setFatalError] = useState(false)
  const retryRef = useRef<(() => void) | null>(null)

  useEffect(() => {
    const video = videoRef.current
    if (!video) return

    let hls: HlsType | undefined
    let cancelled = false
    let pc: RTCPeerConnection | null = null
    let watchdog: ReturnType<typeof setTimeout> | undefined

    const tryPlay = (v: HTMLVideoElement) => {
      v.muted = mutedRef.current
      v.play()
        .then(() => { if (!cancelled) setPlayBlocked(false) })
        .catch((err: unknown) => {
          if (!cancelled && (err as { name?: string })?.name !== 'AbortError') setPlayBlocked(true)
        })
    }

    function setupHLS(v: HTMLVideoElement) {
      import('hls.js').then(({ default: Hls }) => {
        if (cancelled) return

        if (!Hls.isSupported()) {
          v.src = src
          return
        }

        const segSec = segmentSeconds ?? 2
        const liveSyncCount = segSec <= 1 ? 3 : 2
        hlsRef.current = hls = new Hls({
          liveSyncDurationCount: liveSyncCount,
          liveMaxLatencyDurationCount: liveSyncCount * 2,
          maxBufferLength: 10,
          lowLatencyMode: false,
          // Retry manifest when stream isn't ready yet (e.g. right after camera creation)
          manifestLoadingMaxRetry: 30,
          manifestLoadingRetryDelay: 3000,
          manifestLoadingMaxRetryTimeout: 6000,
          xhrSetup(xhr) {
            xhr.setRequestHeader('Authorization', `Bearer ${getToken()}`)
          },
        })
        hls.loadSource(src)
        hls.attachMedia(v)
        hls.on(Hls.Events.MANIFEST_PARSED, () => { tryPlay(v) })
        hls.on(Hls.Events.ERROR, (_e, data) => {
          if (data.fatal) {
            hls?.destroy()
            hls = undefined
            setFatalError(true)
          }
        })
      })
    }

    async function setup(v: HTMLVideoElement) {
      setFatalError(false)
      setPlayBlocked(false)
      v.muted = true

      // Prefer WebRTC (sub-second latency); fall back to HLS when it is
      // unavailable (409 / non-H.264 camera) or the media path never connects.
      // transport='hls' forces HLS by skipping WebRTC entirely.
      if (transport !== 'hls' && cameraId && typeof RTCPeerConnection !== 'undefined') {
        const conn = new RTCPeerConnection()
        pc = conn
        let fellBack = false
        const fallback = () => {
          if (fellBack || cancelled) return
          fellBack = true
          if (watchdog) clearTimeout(watchdog)
          try { conn.close() } catch { /* noop */ }
          if (pc === conn) pc = null
          v.srcObject = null
          setupHLS(v)
        }

        conn.ontrack = (ev) => {
          const [stream] = ev.streams
          if (stream) v.srcObject = stream
        }
        conn.addTransceiver('video', { direction: 'recvonly' })

        try {
          await negotiateWebRTC(cameraId, conn, { token: getToken() ?? undefined })
        } catch {
          fallback()
          return
        }
        if (cancelled) { conn.close(); return }

        conn.onconnectionstatechange = () => {
          if (conn.connectionState === 'connected') {
            if (watchdog) clearTimeout(watchdog)
            tryPlay(v)
          } else if (conn.connectionState === 'failed') {
            fallback()
          }
        }
        // Watchdog: signaling succeeded, but if the media path never connects,
        // fall back to HLS instead of showing a frozen frame.
        watchdog = setTimeout(() => {
          if (conn.connectionState !== 'connected') fallback()
        }, 5000)
        if (conn.connectionState === 'connected') {
          if (watchdog) clearTimeout(watchdog)
          tryPlay(v)
        }
        return
      }

      setupHLS(v)
    }

    retryRef.current = () => setup(video)

    setup(video)

    return () => {
      cancelled = true
      if (watchdog) clearTimeout(watchdog)
      hls?.destroy()
      hlsRef.current = null
      if (pc) {
        try { pc.close() } catch { /* noop */ }
        pc = null
      }
      video.srcObject = null
    }
  }, [src, segmentSeconds, cameraId, transport])

  useEffect(() => {
    mutedRef.current = muted
    if (videoRef.current) videoRef.current.muted = muted
  }, [muted])

  const handleMotionMessage = useCallback((data: string) => {
    const ev = JSON.parse(data) as { score: number; time: string }
    setMotionAlert({ score: ev.score, time: ev.time })
    if (alertTimerRef.current) clearTimeout(alertTimerRef.current)
    alertTimerRef.current = setTimeout(() => setMotionAlert(null), 4000)
  }, [])

  const handleSeekToEvent = useCallback((eventTime: string) => {
    const video = videoRef.current
    if (!video || !video.seekable.length) return
    const offsetSeconds = (Date.now() - new Date(eventTime).getTime()) / 1000
    const target = video.currentTime - offsetSeconds
    const earliest = video.seekable.start(0)
    video.currentTime = Math.max(earliest, target)
  }, [])

  const getStats = useCallback((): HLSStats | null => {
    const h = hlsRef.current
    if (!h) return null
    return {
      bandwidthKbps: Math.round(h.bandwidthEstimate / 1000),
      latencySeconds: h.latency ?? 0,
    }
  }, [])

  const getVideoQuality = useCallback((): VideoPlaybackQuality | null => {
    return videoRef.current?.getVideoPlaybackQuality?.() ?? null
  }, [])

  const getVideoElement = useCallback((): HTMLVideoElement | null => {
    return videoRef.current
  }, [])

  useImperativeHandle(ref, () => ({ seekTo: handleSeekToEvent, getStats, getVideoQuality, getVideoElement }), [handleSeekToEvent, getStats, getVideoQuality, getVideoElement])

  useEffect(() => {
    if (!fatalError) return
    const timer = setTimeout(() => retryRef.current?.(), 2000)
    return () => clearTimeout(timer)
  }, [fatalError])

  useEventSource(
    cameraId ? `/api/cameras/${cameraId}/motion/live` : null,
    handleMotionMessage,
  )

  return (
    <div className={`relative${containerClassName ? ` ${containerClassName}` : ''}`}>
      <video
        ref={videoRef}
        className={className}
        autoPlay
        muted
        playsInline
      />
      {fatalError && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/70">
          <Loader2 className="w-8 h-8 text-gray-400 animate-spin" />
        </div>
      )}
      {!fatalError && playBlocked && (
        <button
          onClick={(e) => { e.stopPropagation(); videoRef.current?.play().catch(() => {}); setPlayBlocked(false) }}
          className="absolute inset-0 flex items-center justify-center bg-black/50 hover:bg-black/40 transition-colors"
          aria-label="Reproduzir"
        >
          <Play className="w-12 h-12 text-white/80 fill-current" />
        </button>
      )}
      {motionAlert && (
        <div className="absolute inset-0 flex flex-col items-center justify-center bg-yellow-400/30 animate-pulse">
          <div className="flex flex-col items-center gap-3 bg-yellow-500/60 backdrop-blur-sm px-6 py-5 rounded-xl">
            <AlertTriangle className="w-8 h-8 text-gray-900" />
            <span className="font-semibold text-gray-900 text-base">Movimento detectado</span>
            <span className="text-xs font-mono text-gray-800">score: {motionAlert.score}</span>
            <div className="flex gap-2 mt-1">
              <button
                onClick={() => {
                  if (alertTimerRef.current) clearTimeout(alertTimerRef.current)
                  setMotionAlert(null)
                  if (onGoToEvent) {
                    onGoToEvent(motionAlert.time)
                  } else {
                    handleSeekToEvent(motionAlert.time)
                  }
                }}
                aria-label="Ir para o momento do evento"
                className="px-3 py-1 text-xs font-medium text-gray-900 bg-yellow-300/70 hover:bg-yellow-300 rounded transition-colors cursor-pointer"
              >
                Ir para este momento
              </button>
              <button
                onClick={() => {
                  if (alertTimerRef.current) clearTimeout(alertTimerRef.current)
                  setMotionAlert(null)
                }}
                aria-label="Fechar alerta"
                className="px-3 py-1 text-xs font-medium text-gray-900 bg-yellow-300/70 hover:bg-yellow-300 rounded transition-colors cursor-pointer"
              >
                Fechar
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
})

export default HLSPlayer
