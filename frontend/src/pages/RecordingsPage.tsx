import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { format } from 'date-fns'
import AppLayout from '../components/AppLayout'
import DatePicker from '../components/DatePicker'
import { authHeaders, onUnauthorized, getToken } from '../auth'

const CAT_LABEL: Record<string, string> = { movimento: 'Movimento', pessoa: 'Pessoa', ia: 'IA', estados: 'Estados' }
const CAT_COLOR: Record<string, string> = { movimento: 'bg-amber-400', pessoa: 'bg-red-500', ia: 'bg-violet-500', estados: 'bg-green-500' }
const CAT_FILTERS = ['todos', 'movimento', 'pessoa', 'ia', 'estados'] as const

interface CameraOption { id: string; name: string }
interface Moment {
  camera_id: string
  camera_name: string
  time: string
  kind: 'motion' | 'state'
  label?: string
  category: string
  frame?: string
  score: number
}

const pad = (n: number) => String(n).padStart(2, '0')

// momentThumb resolve a URL do thumbnail: estado já vem como caminho absoluto
// (/recordings/state_history/...); movimento é só o nome do arquivo, montado a partir
// da câmera + dia UTC do instante.
function momentThumb(m: Moment): string | null {
  if (!m.frame) return null
  if (m.frame.startsWith('/')) return `${m.frame}?token=${getToken()}`
  const d = new Date(m.time)
  const dir = `${d.getUTCFullYear()}/${pad(d.getUTCMonth() + 1)}/${pad(d.getUTCDate())}`
  return `/recordings/${m.camera_id}/${dir}/${m.frame}?token=${getToken()}`
}

export default function RecordingsPage() {
  const navigate = useNavigate()
  const [cameras, setCameras] = useState<CameraOption[]>([])
  const [selectedCams, setSelectedCams] = useState<Set<string>>(new Set())
  const [category, setCategory] = useState<string>('todos')
  const [date, setDate] = useState<Date>(new Date())
  const [moments, setMoments] = useState<Moment[]>([])
  const [hasMore, setHasMore] = useState(false)
  const [page, setPage] = useState(1)
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    fetch('/api/cameras', { headers: authHeaders() })
      .then(r => { if (r.status === 401) { onUnauthorized(); return null } return r.json() })
      .then((list: CameraOption[] | null) => { if (list) setCameras(list) })
      .catch(() => {})
  }, [])

  useEffect(() => {
    let cancelled = false
    const params = new URLSearchParams({ date: format(date, 'yyyy-MM-dd'), page: String(page), limit: '120' })
    if (category !== 'todos') params.set('category', category)
    if (selectedCams.size > 0) params.set('cameras', [...selectedCams].join(','))
    fetch(`/api/moments?${params}`, { headers: authHeaders() })
      .then(r => { if (r.status === 401) { onUnauthorized(); return null } return r.json() })
      .then(d => {
        if (cancelled || !d) return
        setMoments(prev => (page === 1 ? d.moments : [...prev, ...d.moments]))
        setHasMore(d.hasMore)
        setLoaded(true)
      })
      .catch(() => {})
    return () => { cancelled = true }
  }, [date, category, page, selectedCams])

  const toggleCam = (id: string) => {
    setSelectedCams(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
    setPage(1)
  }

  return (
    <AppLayout>
      <div className="flex items-end justify-between mb-4 gap-4 flex-wrap">
        <div>
          <h2 className="text-2xl font-bold text-foreground">Gravações</h2>
          <p className="text-sm text-muted mt-1">Momentos das câmeras — clique para abrir na gravação.</p>
        </div>
        <DatePicker id="recordings-day-picker" value={date} onChange={d => { setDate(d); setPage(1) }} disableFuture align="right" />
      </div>

      {/* Filtro de categoria */}
      <div id="recordings-category-chips" className="flex flex-wrap items-center gap-1.5 mb-2">
        {CAT_FILTERS.map(c => (
          <button
            key={c}
            id={`recordings-cat-${c}`}
            onClick={() => { setCategory(c); setPage(1) }}
            className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs transition-colors ${
              category === c ? 'bg-primary text-primary-foreground' : 'bg-surface-2 text-muted hover:text-foreground'
            }`}
          >
            {c !== 'todos' && <span className={`w-1.5 h-1.5 rounded-full ${CAT_COLOR[c]}`} />}
            {c === 'todos' ? 'Todos' : CAT_LABEL[c]}
          </button>
        ))}
      </div>

      {/* Filtro de câmera (multi; nenhuma marcada = todas) */}
      {cameras.length > 1 && (
        <div id="recordings-camera-chips" className="flex flex-wrap items-center gap-1.5 mb-4">
          {cameras.map(c => (
            <button
              key={c.id}
              id={`recordings-cam-${c.id}`}
              onClick={() => toggleCam(c.id)}
              className={`rounded-full px-2.5 py-1 text-xs transition-colors ${
                selectedCams.has(c.id) ? 'bg-primary text-primary-foreground' : 'bg-surface-2 text-muted hover:text-foreground'
              }`}
            >
              {c.name}
            </button>
          ))}
        </div>
      )}

      {moments.length === 0 && loaded ? (
        <p className="text-sm text-muted">Nenhum momento nesta data.</p>
      ) : (
        <div id="recordings-grid" className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-3">
          {moments.map((m, i) => {
            const thumb = momentThumb(m)
            return (
              <button
                key={`${m.camera_id}-${m.time}-${i}`}
                id={`moment-${i}`}
                onClick={() => navigate(`/cameras/${m.camera_id}`, { state: { eventTime: m.time, showRecordings: true } })}
                className="bg-surface border border-border rounded-lg overflow-hidden text-left hover:border-primary/50 transition-colors"
              >
                {thumb ? (
                  <img src={thumb} alt={m.category} className="w-full aspect-video object-cover bg-black" loading="lazy" />
                ) : (
                  <div className="w-full aspect-video bg-surface-2 flex items-center justify-center text-[10px] text-faint">sem prévia</div>
                )}
                <div className="px-2 py-1.5">
                  <div className="flex items-center gap-1.5">
                    <span className={`w-2 h-2 rounded-full shrink-0 ${CAT_COLOR[m.category] ?? 'bg-border'}`} />
                    <span className="text-xs font-medium text-foreground truncate">{m.camera_name}</span>
                  </div>
                  <p className="text-[10px] text-muted tabular-nums">{format(new Date(m.time), 'dd/MM HH:mm')}</p>
                </div>
              </button>
            )
          })}
        </div>
      )}

      {hasMore && (
        <div className="flex justify-center mt-4">
          <button
            id="recordings-load-more"
            onClick={() => setPage(p => p + 1)}
            className="px-3 py-1.5 rounded text-xs bg-surface-2 text-muted hover:text-foreground transition-colors"
          >
            Carregar mais
          </button>
        </div>
      )}
    </AppLayout>
  )
}
