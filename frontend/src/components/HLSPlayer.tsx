import { forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState } from 'react'
import type HlsType from 'hls.js'
import { getToken } from '../auth'
import { useEventSource } from '../hooks/useEventSource'

export interface HLSStats {
  bandwidthKbps: number
  latencySeconds: number
}

export interface HLSPlayerHandle {
  seekTo: (isoTime: string) => void
  getStats: () => HLSStats | null
  getVideoQuality: () => VideoPlaybackQuality | null
}

interface HLSPlayerProps {
  src: string
  className?: string
  cameraId?: string
  muted?: boolean
  onGoToEvent?: (eventTime: string) => void
}

interface MotionAlert {
  score: number
  time: string
}

const HLSPlayer = forwardRef<HLSPlayerHandle, HLSPlayerProps>(function HLSPlayer({ src, className, cameraId, muted = true, onGoToEvent }, ref) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const wrapperRef = useRef<HTMLDivElement>(null)
  const hlsRef = useRef<HlsType | null>(null)
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

    function setup(v: HTMLVideoElement) {
      setFatalError(false)
      setPlayBlocked(false)

      import('hls.js').then(({ default: Hls }) => {
        if (cancelled) return

        if (!Hls.isSupported()) {
          v.src = src
          return
        }

        hlsRef.current = hls = new Hls({
          liveSyncDurationCount: 3,
          liveMaxLatencyDurationCount: 6,
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
        hls.on(Hls.Events.MANIFEST_PARSED, () => {
          v.play().catch(() => setPlayBlocked(true))
        })
        hls.on(Hls.Events.ERROR, (_e, data) => {
          if (data.fatal) {
            hls?.destroy()
            hls = undefined
            setFatalError(true)
          }
        })
      })
    }

    retryRef.current = () => setup(video)

    setup(video)

    return () => {
      cancelled = true
      hls?.destroy()
      hlsRef.current = null
    }
  }, [src])

  useEffect(() => {
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

  useImperativeHandle(ref, () => ({ seekTo: handleSeekToEvent, getStats, getVideoQuality }), [handleSeekToEvent, getStats, getVideoQuality])

  useEffect(() => {
    if (!fatalError) return
    const timer = setTimeout(() => retryRef.current?.(), 2000)
    return () => clearTimeout(timer)
  }, [fatalError])

  useEventSource(
    cameraId ? `/api/cameras/${cameraId}/motion/live` : null,
    handleMotionMessage,
  )

  function handleFullscreen(e: React.MouseEvent) {
    e.stopPropagation()
    if (document.fullscreenElement) {
      document.exitFullscreen().catch(() => {})
    } else {
      wrapperRef.current?.requestFullscreen().catch(() => {})
    }
  }

  return (
    <div ref={wrapperRef} className="relative">
      <video
        ref={videoRef}
        className={className}
        muted
        playsInline
      />
      {fatalError && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/70">
          <svg className="w-8 h-8 text-gray-400 animate-spin" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
          </svg>
        </div>
      )}
      {!fatalError && playBlocked && (
        <button
          onClick={(e) => { e.stopPropagation(); videoRef.current?.play().catch(() => {}); setPlayBlocked(false) }}
          className="absolute inset-0 flex items-center justify-center bg-black/50 hover:bg-black/40 transition-colors"
          aria-label="Reproduzir"
        >
          <svg className="w-12 h-12 text-white/80" fill="currentColor" viewBox="0 0 24 24">
            <path d="M8 5v14l11-7z" />
          </svg>
        </button>
      )}
      {motionAlert && (
        <div className="absolute inset-0 flex flex-col items-center justify-center bg-yellow-400/30 animate-pulse">
          <div className="flex flex-col items-center gap-3 bg-yellow-500/60 backdrop-blur-sm px-6 py-5 rounded-xl">
            <svg xmlns="http://www.w3.org/2000/svg" className="w-8 h-8 text-gray-900" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
            </svg>
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
      <button
        onClick={handleFullscreen}
        aria-label="Tela inteira"
        className="absolute bottom-2 right-2 p-1.5 rounded bg-black/40 text-white/70 hover:text-white hover:bg-black/60 transition-colors"
      >
        <svg xmlns="http://www.w3.org/2000/svg" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 8V6a2 2 0 012-2h2M4 16v2a2 2 0 002 2h2m8-16h2a2 2 0 012 2v2m0 8v2a2 2 0 01-2 2h-2" />
        </svg>
      </button>
    </div>
  )
})

export default HLSPlayer
