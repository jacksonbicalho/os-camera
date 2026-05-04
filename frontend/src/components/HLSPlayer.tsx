import { useEffect, useRef, useState } from 'react'
import type HlsType from 'hls.js'
import { getToken } from '../auth'

interface HLSPlayerProps {
  src: string
  className?: string
}

export default function HLSPlayer({ src, className }: HLSPlayerProps) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const wrapperRef = useRef<HTMLDivElement>(null)
  const [muted, setMuted] = useState(true)

  useEffect(() => {
    const video = videoRef.current
    if (!video) return

    let hls: HlsType | undefined
    let cancelled = false

    import('hls.js').then(({ default: Hls }) => {
      if (cancelled) return

      if (!Hls.isSupported()) {
        video.src = src
        return
      }

      hls = new Hls({
        liveSyncDurationCount: 3,
        liveMaxLatencyDurationCount: 6,
        maxBufferLength: 10,
        lowLatencyMode: false,
        xhrSetup(xhr) {
          xhr.setRequestHeader('Authorization', `Bearer ${getToken()}`)
        },
      })
      hls.loadSource(src)
      hls.attachMedia(video)
      hls.on(Hls.Events.MANIFEST_PARSED, () => video.play().catch(() => {}))
    })

    return () => {
      cancelled = true
      hls?.destroy()
    }
  }, [src])

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
}
