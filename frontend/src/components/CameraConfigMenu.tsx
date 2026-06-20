import { useState, useRef, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { Settings, ChevronDown } from './Icons'

// Seções de configuração de uma câmera (espelha CameraSettingsTabs + a rota de
// edição). `key` vira o id de teste/automação de cada item.
const SECTIONS: { key: string; label: string; path: (id: string) => string }[] = [
  { key: 'detail', label: 'Câmera', path: id => `/settings/cameras/${id}` },
  { key: 'edit', label: 'Editar', path: id => `/settings/cameras/edit/${id}` },
  { key: 'motion', label: 'Movimento', path: id => `/settings/cameras/motion/${id}` },
  { key: 'zones', label: 'Zonas', path: id => `/settings/cameras/zones/${id}` },
  { key: 'analysis', label: 'Análise', path: id => `/settings/cameras/analysis/${id}` },
  { key: 'states', label: 'Estados', path: id => `/settings/cameras/states/${id}` },
]

interface Props {
  cameraId: string
  showIcon?: boolean
  showLabel?: boolean
}

// CameraConfigMenu: botão no header do player que abre um dropdown com as seções
// de configuração da câmera. Fecha ao selecionar, clicar fora e no Esc.
export default function CameraConfigMenu({ cameraId, showIcon = true, showLabel = true }: Props) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setOpen(false) }
    document.addEventListener('mousedown', onDown)
    window.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDown)
      window.removeEventListener('keydown', onKey)
    }
  }, [open])

  return (
    <div ref={ref} className="relative">
      <button
        id="camera-config-menu"
        onClick={() => setOpen(o => !o)}
        title="Configurar câmera"
        aria-haspopup="menu"
        aria-expanded={open}
        className="flex items-center gap-1 px-1 py-1 text-muted hover:text-foreground transition-colors cursor-pointer"
      >
        {showIcon && <Settings className="w-4 h-4" />}
        {showLabel && <span className="text-[11px] leading-none">Câmera</span>}
        <ChevronDown className="w-3 h-3" />
      </button>

      {open && (
        <div
          id="camera-config-menu-list"
          role="menu"
          className="absolute right-0 top-full mt-1 z-50 w-44 bg-surface border border-border rounded shadow-lg py-1"
        >
          {SECTIONS.map(s => (
            <Link
              key={s.key}
              id={`camera-config-item-${s.key}`}
              role="menuitem"
              to={s.path(cameraId)}
              onClick={() => setOpen(false)}
              className="block px-3 py-1.5 text-sm text-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
            >
              {s.label}
            </Link>
          ))}
        </div>
      )}
    </div>
  )
}
