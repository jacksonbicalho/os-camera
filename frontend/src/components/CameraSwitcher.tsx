import { useState, useRef, useEffect } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { authHeaders, onUnauthorized } from '../auth'
import { Cctv, ChevronDown } from './Icons'
import { Button } from './ui/button'

interface CameraItem {
  id: string
  name: string
}

// CameraSwitcher — dropdown "Câmeras" no header do player (redesign do Escopo B).
// Lista GET /api/cameras ao abrir e navega para /camera/live/{id} ao escolher.
// Reaviva a troca rápida que ficava no sidebar antes do redesign do nav rail.
export default function CameraSwitcher() {
  const navigate = useNavigate()
  const { id } = useParams<{ id?: string }>()
  const [open, setOpen] = useState(false)
  const [cameras, setCameras] = useState<CameraItem[]>([])
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])

  function toggle() {
    if (!open) {
      fetch('/api/cameras', { headers: authHeaders() })
        .then(res => { if (res.status === 401) { onUnauthorized(); return null } return res.json() })
        .then(data => { if (Array.isArray(data)) setCameras(data) })
        .catch(() => {})
    }
    setOpen(o => !o)
  }

  return (
    <div className="relative shrink-0" ref={ref}>
      <Button id="camera-switcher" variant="ghost" size="sm" className="gap-1.5" onClick={toggle} title="Trocar de câmera">
        <Cctv className="w-4 h-4" />
        <span>Câmeras</span>
        <ChevronDown className="w-3.5 h-3.5" />
      </Button>
      {open && (
        <div id="camera-switcher-menu" className="absolute left-0 mt-1 z-30 min-w-44 bg-surface border border-border rounded-lg shadow-xl py-1">
          {cameras.length === 0 ? (
            <div className="px-3 py-2 text-sm text-muted-foreground">Nenhuma câmera</div>
          ) : cameras.map(cam => (
            <button
              key={cam.id}
              onClick={() => { setOpen(false); navigate(`/camera/live/${cam.id}`, { replace: true, state: { goLive: Date.now() } }) }}
              className={`block w-full text-left px-3 py-2 text-sm transition-colors truncate ${
                cam.id === id ? 'text-primary font-medium' : 'text-foreground hover:bg-accent hover:text-accent-foreground'
              }`}
            >
              {cam.name}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
