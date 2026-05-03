import { useEffect, useRef } from 'react'
import Hls from 'hls.js'
import { getToken } from '../auth'

interface HLSPlayerProps {
  src: string
  className?: string
}

export default function HLSPlayer({ src, className }: HLSPlayerProps) {
  const videoRef = useRef<HTMLVideoElement>(null)

  useEffect(() => {
    const video = videoRef.current
    if (!video) return

    if (!Hls.isSupported()) {
      video.src = src
      return
    }

    const hls = new Hls({
      xhrSetup(xhr) {
        xhr.setRequestHeader('Authorization', `Bearer ${getToken()}`)
      },
    })
    hls.loadSource(src)
    hls.attachMedia(video)
    hls.on(Hls.Events.MANIFEST_PARSED, () => video.play().catch(() => {}))

    return () => hls.destroy()
  }, [src])

  return (
    <video
      ref={videoRef}
      className={className}
      controls
      muted
      playsInline
    />
  )
}
