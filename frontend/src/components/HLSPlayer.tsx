import { forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState } from 'react'
import type HlsType from 'hls.js'
import { getToken } from '../auth'
import { useEventSource } from '../hooks/useEventSource'

export interface HLSPlayerHandle {
  seekTo: (isoTime: string) => void
}

interface HLSPlayerProps {
  src: string
  className?: string
  cameraId?: string
  onGoToEvent?: (eventTime: string) => void
}

interface MotionAlert {
  score: number
  time: string
}

const HLSPlayer = forwardRef<HLSPlayerHandle, HLSPlayerProps>(function HLSPlayer({ src, className, cameraId, onGoToEvent }, ref) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const wrapperRef = useRef<HTMLDivElement>(null)
  const [muted, setMuted] = useState(true)
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

        hls = new Hls({
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
            setFatalError(true)
          }
        })
      })
    }

    retryRef.current = () => {
      hls?.destroy()
      setup(video)
    }

    setup(video)

    return () => {
      cancelled = true
      hls?.destroy()
    }
  }, [src])

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

  useImperativeHandle(ref, () => ({ seekTo: handleSeekToEvent }), [handleSeekToEvent])

  useEventSource(
    cameraId ? `/api/cameras/${cameraId}/motion/live` : null,
    handleMotionMessage,
  )

  function handleMute(e: React.MouseEvent) {
    e.stopPropagation()
    const video = videoRef.current
    if (!video) return
    video.muted = !video.muted
    setMuted(video.muted)
  }

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
        <div className="absolute inset-0 flex flex-col items-center justify-center bg-black/70 gap-3">
          <span className="text-sm text-gray-300">Stream indisponível</span>
          <button
            onClick={(e) => { e.stopPropagation(); retryRef.current?.() }}
            className="px-3 py-1.5 text-xs font-medium bg-blue-600 hover:bg-blue-500 text-white rounded transition-colors"
          >
            Tentar novamente
          </button>
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
        onClick={handleMute}
        aria-label={muted ? 'Ativar áudio' : 'Silenciar'}
        className="absolute bottom-2 right-10 p-1.5 rounded bg-black/40 text-white/70 hover:text-white hover:bg-black/60 transition-colors"
      >
        {muted ? (
          <svg xmlns="http://www.w3.org/2000/svg" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2" />
          </svg>
        ) : (
          <svg xmlns="http://www.w3.org/2000/svg" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.536 8.464a5 5 0 010 7.072M12 6v12m-3.536-9.536a5 5 0 000 7.072M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
          </svg>
        )}
      </button>
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
