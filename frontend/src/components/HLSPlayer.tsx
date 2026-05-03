import { useEffect, useRef } from 'react'
import Hls from 'hls.js'
import { getToken } from '../auth'

interface HLSPlayerProps {
  src: string
  className?: string
}

export default function HLSPlayer({ src, className }: HLSPlayerProps) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const wrapperRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const video = videoRef.current
    if (!video) return

    if (!Hls.isSupported()) {
      video.src = src
      return
    }

    const hls = new Hls({
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

    return () => hls.destroy()
  }, [src])

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
