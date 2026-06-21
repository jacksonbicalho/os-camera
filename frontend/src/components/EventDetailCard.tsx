import type { MotionEvent } from '../pages/cameraUtils'
import { eventCategory, eventTitle } from '../pages/eventCategory'
import { Play } from './Icons'
import { Button } from './ui/button'

const CAT_COLOR: Record<string, string> = {
  movimento: 'text-amber-400',
  pessoa: 'text-red-400',
  ia: 'text-violet-400',
  estados: 'text-green-400',
}

function mmss(totalSeconds: number): string {
  const s = Math.max(0, Math.round(totalSeconds))
  return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`
}

interface EventDetailCardProps {
  event: MotionEvent | null
  cameraName: string
  durationSeconds: number
  thumbSrc: string | null
  onPlay: () => void
  onDownload: () => void
  onMark: () => void
}

// EventDetailCard — card de detalhe do evento selecionado (aba "Linha do tempo"):
// thumbnail com play, Tipo/Confiança/Duração/Câmera e ações Reproduzir/Download/Marcar.
export default function EventDetailCard({ event, cameraName, durationSeconds, thumbSrc, onPlay, onDownload, onMark }: EventDetailCardProps) {
  if (!event) {
    return (
      <div id="event-detail-card" className="shrink-0 border-b border-border px-3 py-4 text-center text-xs text-muted">
        Selecione um evento na lista para ver os detalhes.
      </div>
    )
  }

  const cat = eventCategory(event)
  return (
    <div id="event-detail-card" className="shrink-0 border-b border-border p-3 flex flex-col gap-3">
      {/* Thumbnail ocupando toda a largura */}
      <button
        id="event-detail-play"
        onClick={onPlay}
        className="relative w-full aspect-video rounded overflow-hidden border border-border bg-surface-2 group"
        title="Reproduzir"
      >
        {thumbSrc && <img src={thumbSrc} alt="" className="w-full h-full object-cover" />}
        <span className="absolute inset-0 flex items-center justify-center">
          <span className="w-10 h-10 rounded-full bg-black/60 flex items-center justify-center text-white group-hover:bg-primary transition-colors">
            <Play className="w-4 h-4 fill-current" />
          </span>
        </span>
      </button>

      {/* Informações organizadas abaixo */}
      <span className={`text-sm font-medium ${CAT_COLOR[cat] ?? 'text-foreground'}`}>{eventTitle(event)}</span>

      <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs min-w-0">
        <dt className="text-faint">Confiança</dt>
        <dd className="text-foreground tabular-nums">{(event.score * 100).toFixed(0)}%</dd>
        <dt className="text-faint">Duração</dt>
        <dd className="text-foreground tabular-nums">{mmss(durationSeconds)}</dd>
        <dt className="text-faint">Câmera</dt>
        <dd className="text-foreground truncate">{cameraName}</dd>
      </dl>

      <div className="flex items-center gap-2">
        <Button id="event-detail-reproduzir" size="sm" className="gap-1.5" onClick={onPlay}>
          <Play className="w-3.5 h-3.5 fill-current" /> Reproduzir
        </Button>
        <Button id="event-detail-download" size="sm" variant="outline" onClick={onDownload}>Download</Button>
        <Button id="event-detail-mark" size="sm" variant="outline" onClick={onMark}>Marcar</Button>
      </div>
    </div>
  )
}
