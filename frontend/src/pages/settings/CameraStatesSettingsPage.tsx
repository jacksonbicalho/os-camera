import { useState, useEffect, useCallback, useRef } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { DayPicker } from 'react-day-picker'
import { format } from 'date-fns'
import { ptBR } from 'date-fns/locale'
import 'react-day-picker/style.css'
import SettingsLayout from '../../components/SettingsLayout'
import CameraSettingsTabs from '../../components/CameraSettingsTabs'
import ConfirmDialog from '../../components/ConfirmDialog'
import BboxCanvas, { type BboxRect } from '../../components/BboxCanvas'
import { authHeaders, onUnauthorized, getRole, getToken } from '../../auth'
import { Button } from '@/components/ui/button'
import { Plus, Pencil, Trash2, Zap, Loader2, Camera as CameraIcon, CalendarDays, Film, X } from '../../components/Icons'
import {
  type StateClassifier,
  bboxToCrop,
  cropToBbox,
  validateClassifier,
} from './stateClassifier'
import { loadPicked, loadPickerScroll, savePicked, savePickerScroll } from './eventPickerMemory'
import { eventCleanFrameURL } from './stateEventFrames'

function emptyClassifier(): StateClassifier {
  return {
    name: '',
    threshold: 0.8,
    trigger_motion: false,
    trigger_interval_seconds: 10,
    crop_x: 0.3, crop_y: 0.3, crop_w: 0.4, crop_h: 0.4,
    min_consecutive: 3,
    enabled: true,
    classes: ['fechado', 'aberto'],
    notify_enabled: true,
    footer_enabled: false,
    notify_user_ids: [],
    footer_user_ids: [],
  }
}

// captureFromUrl carrega uma imagem (snapshot ao vivo, frame de evento ou um
// data URL) e devolve {crop, source}: `crop` é o recorte da região (vai pro
// thumbnail e pro treino); `source` é o frame inteiro como data URL (para clicar
// no thumb e trazê-lo de volta ao quadro principal).
function captureFromUrl(url: string, c: StateClassifier): Promise<{ crop: string; source: string }> {
  return new Promise((resolve, reject) => {
    const img = new Image()
    img.onload = () => {
      const ctx = document.createElement('canvas').getContext('2d')
      // frame inteiro
      const full = document.createElement('canvas')
      full.width = img.naturalWidth
      full.height = img.naturalHeight
      full.getContext('2d')?.drawImage(img, 0, 0)
      // recorte da região
      const sx = c.crop_x * img.naturalWidth
      const sy = c.crop_y * img.naturalHeight
      const sw = Math.max(1, c.crop_w * img.naturalWidth)
      const sh = Math.max(1, c.crop_h * img.naturalHeight)
      const cropCanvas = document.createElement('canvas')
      cropCanvas.width = Math.round(sw)
      cropCanvas.height = Math.round(sh)
      const cctx = cropCanvas.getContext('2d')
      if (!ctx || !cctx) { reject(new Error('no canvas')); return }
      cctx.drawImage(img, sx, sy, sw, sh, 0, 0, cropCanvas.width, cropCanvas.height)
      resolve({
        crop: cropCanvas.toDataURL('image/jpeg', 0.9),
        source: full.toDataURL('image/jpeg', 0.9),
      })
    }
    img.onerror = () => reject(new Error('image load failed'))
    img.src = url
  })
}

function liveSnapshotURL(cameraId: string): string {
  return `/api/cameras/${cameraId}/snapshot?token=${getToken()}&t=${Date.now()}`
}

interface EventItem { time: string; frame: string }

function todayStr(): string {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

// eventSnapshotURL replica o caminho do snapshot ANOTADO de um evento (rápido,
// usado nos thumbs do carrossel para navegar).
function eventSnapshotURL(cameraId: string, ev: EventItem): string {
  const d = new Date(ev.time)
  const dateDir = `${d.getUTCFullYear()}/${String(d.getUTCMonth() + 1).padStart(2, '0')}/${String(d.getUTCDate()).padStart(2, '0')}`
  return `/recordings/${cameraId}/${dateDir}/${ev.frame}?token=${getToken()}`
}

// eventFrameURL extrai um frame LIMPO (sem o bbox de movimento) da gravação no
// instante do evento — é o que vira a amostra/treino ao escolher.
function eventFrameURL(cameraId: string, ev: EventItem): string {
  return `/api/cameras/${cameraId}/event-frame?time=${encodeURIComponent(ev.time)}&token=${getToken()}`
}

// imageLoads resolve true se a URL carrega como imagem, false caso contrário.
function imageLoads(url: string): Promise<boolean> {
  return new Promise(res => {
    const img = new Image()
    img.onload = () => res(true)
    img.onerror = () => res(false)
    img.src = url
  })
}

// loadEventImageURL devolve o frame LIMPO da gravação; se não houver gravação
// cobrindo o instante (extração falha), cai no snapshot do evento (_motion.jpg,
// com a caixa de movimento) para a adição não falhar calada. `fellBack` sinaliza.
async function loadEventImageURL(cameraId: string, ev: EventItem): Promise<{ url: string; fellBack: boolean }> {
  // 1. frame limpo salvo junto do snapshot (instante exato, sem defasagem)
  const cleanFrame = eventCleanFrameURL(cameraId, ev)
  if (await imageLoads(cleanFrame)) return { url: cleanFrame, fellBack: false }
  // 2. eventos antigos sem _frame.jpg: extrai da gravação
  const extracted = eventFrameURL(cameraId, ev)
  if (await imageLoads(extracted)) return { url: extracted, fellBack: false }
  // 3. último recurso: snapshot anotado (com a caixa de movimento)
  return { url: eventSnapshotURL(cameraId, ev), fellBack: true }
}

export default function CameraStatesSettingsPage() {
  const { id, cid } = useParams<{ id: string; cid?: string }>()
  const navigate = useNavigate()
  const isAdmin = getRole() === 'admin'
  const [items, setItems] = useState<StateClassifier[]>([])
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<StateClassifier | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [deleteId, setDeleteId] = useState<number | null>(null)
  const [states, setStates] = useState<Record<number, string>>({})
  const [historyFor, setHistoryFor] = useState<StateClassifier | null>(null)
  const [searchParams, setSearchParams] = useSearchParams()

  // Estado atual de cada classificador, em poll (~5s) — atualiza ao runner mudar.
  useEffect(() => {
    if (editing || items.length === 0) return
    let cancelled = false
    const fetchStates = () => Promise.all(items.map(c =>
      fetch(`/api/cameras/${id}/classifiers/${c.id}/state`, { headers: authHeaders() })
        .then(r => r.ok ? r.json() : null)
        .then(d => [c.id!, d?.state ?? ''] as const)
        .catch(() => [c.id!, ''] as const)
    )).then(entries => { if (!cancelled) setStates(Object.fromEntries(entries)) })
    fetchStates()
    const iv = setInterval(fetchStates, 5000)
    return () => { cancelled = true; clearInterval(iv) }
  }, [items, id, editing])

  const reload = useCallback(async () => {
    const res = await fetch(`/api/settings/cameras/${id}/classifiers`, { headers: authHeaders() })
    if (res.status === 401) { onUnauthorized(); return }
    if (res.ok) setItems(await res.json())
  }, [id])

  useEffect(() => {
    fetch(`/api/settings/cameras/${id}/classifiers`, { headers: authHeaders() })
      .then(res => {
        if (res.status === 401) { onUnauthorized(); return [] }
        return res.ok ? res.json() : []
      })
      .then((data: StateClassifier[]) => {
        setItems(data)
        // O :cid da rota é a fonte de verdade da edição (deep-link/reload/back/
        // forward): com cid, edita o classificador; sem cid, fecha o form. É callback
        // async, então não cai no lint set-state-in-effect. (O "Novo" não tem cid e
        // não dispara este efeito, então não é afetado.)
        setEditing(cid ? data.find(c => String(c.id) === cid) ?? null : null)
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [id, cid])

  // Deep-link do rodapé: ?history={cid} abre direto a view de Histórico do
  // classificador. setState dentro do .then (async) para não cair no lint
  // set-state-in-effect (mesmo padrão do efeito de edição acima).
  useEffect(() => {
    const h = searchParams.get('history')
    if (!h || items.length === 0) return
    Promise.resolve().then(() => {
      const c = items.find(x => String(x.id) === h)
      if (c) setHistoryFor(c)
    })
  }, [searchParams, items])

  // closeHistory volta pra lista e limpa o ?history pra não reabrir no próximo render.
  const closeHistory = useCallback(() => {
    setHistoryFor(null)
    if (searchParams.has('history')) {
      searchParams.delete('history')
      setSearchParams(searchParams, { replace: true })
    }
  }, [searchParams, setSearchParams])

  async function remove() {
    if (deleteId == null) return
    await fetch(`/api/settings/cameras/${id}/classifiers/${deleteId}`, { method: 'DELETE', headers: authHeaders() })
    setDeleteId(null)
    await reload()
  }

  // Treino a partir das amostras já salvas (corpo vazio): o servidor recorta os
  // frames persistidos e dispara o YOLO. `trainingId` = id em treino, 'all' no lote.
  const [trainingId, setTrainingId] = useState<number | 'all' | null>(null)
  const [trainMsg, setTrainMsg] = useState('')

  async function trainOne(cid: number): Promise<string> {
    const res = await fetch(`/api/settings/cameras/${id}/classifiers/${cid}/train`, {
      method: 'POST', headers: { ...authHeaders(), 'Content-Type': 'application/json' }, body: '{}',
    })
    return res.ok ? '' : ((await res.text()).trim() || `erro ${res.status}`)
  }

  async function handleTrainOne(c: StateClassifier) {
    if (trainingId != null) return
    setTrainingId(c.id!); setTrainMsg('')
    const err = await trainOne(c.id!)
    setTrainMsg(err ? `Falha ao treinar "${c.name}": ${err}` : `Treino de "${c.name}" iniciado.`)
    setTrainingId(null)
  }

  async function handleTrainAll() {
    if (trainingId != null || items.length === 0) return
    setTrainingId('all'); setTrainMsg('Treinando todos…')
    let ok = 0
    const fails: string[] = []
    for (const c of items) {
      const err = await trainOne(c.id!)
      if (err) fails.push(c.name); else ok++
    }
    setTrainMsg(fails.length
      ? `Treino iniciado em ${ok}; falhou: ${fails.join(', ')}`
      : `Treino iniciado em ${ok} classificador(es).`)
    setTrainingId(null)
  }

  if (!isAdmin) {
    return (
      <SettingsLayout>
        <CameraSettingsTabs id={id!} active="states" />
        <p className="text-muted-foreground text-sm">Apenas administradores.</p>
      </SettingsLayout>
    )
  }

  return (
    <SettingsLayout>
      <CameraSettingsTabs id={id!} active="states" />

      {error && (
        <div className="mb-4 px-3 py-2 bg-red-900/30 border border-red-700/50 rounded text-xs text-red-400">{error}</div>
      )}

      {editing ? (
        <ClassifierForm
          cameraId={id!}
          value={editing}
          onChange={setEditing}
          onDone={() => { setEditing(null); if (cid) navigate(`/settings/cameras/states/${id}`); else reload() }}
          onCancel={() => { setEditing(null); setError(null); if (cid) navigate(`/settings/cameras/states/${id}`) }}
        />
      ) : historyFor ? (
        <ClassifierHistory cameraId={id!} classifier={historyFor} onBack={closeHistory} />
      ) : (
        <>
          <div className="flex items-center justify-between mb-4">
            <p className="text-sm text-muted-foreground">Classificadores de estado (recorte fixo → estado).</p>
            <div className="flex items-center gap-2">
              <Button
                id="state-train-all"
                variant="outline"
                disabled={trainingId != null || items.length === 0}
                title="Treina todos os classificadores a partir das amostras já salvas"
                onClick={handleTrainAll}
              >
                {trainingId === 'all' ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Zap className="w-3.5 h-3.5" />} Treinar todos
              </Button>
              <Button id="state-classifier-new" onClick={() => { setEditing(emptyClassifier()); setError(null) }}>
                <Plus className="w-3.5 h-3.5" /> Novo classificador
              </Button>
            </div>
          </div>

          {trainMsg && <p id="state-train-msg" className="text-xs text-muted-foreground mb-3">{trainMsg}</p>}

          {loading ? (
            <p className="text-muted-foreground text-sm">Carregando...</p>
          ) : items.length === 0 ? (
            <p className="text-muted-foreground text-sm">Nenhum classificador configurado.</p>
          ) : (
            <div className="flex flex-col gap-2">
              {items.map(c => (
                <div key={c.id} className="bg-surface border border-border rounded-lg px-4 py-3 flex items-center gap-3">
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium text-foreground truncate">{c.name}</p>
                    <p className="text-xs text-muted-foreground">
                      {c.classes.join(' · ')} — a cada {c.trigger_interval_seconds}s · limiar {c.threshold}
                      {!c.enabled && ' · desativado'}
                    </p>
                  </div>
                  <span
                    className={`px-2 py-0.5 text-xs rounded border tabular-nums shrink-0 ${
                      states[c.id!] ? 'bg-primary/15 text-primary border-primary/30' : 'bg-surface-2 text-muted-foreground border-border'
                    }`}
                    title="Estado atual"
                  >
                    {states[c.id!] || '—'}
                  </span>
                  <Button
                    id={`state-history-${c.id}`}
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 text-muted-foreground hover:text-primary"
                    title="Histórico de estados"
                    onClick={() => setHistoryFor(c)}
                  >
                    <CalendarDays className="w-4 h-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 text-muted-foreground hover:text-primary"
                    title="Treinar agora (usa as amostras já salvas)"
                    disabled={trainingId != null}
                    onClick={() => handleTrainOne(c)}
                  >
                    {trainingId === c.id ? <Loader2 className="w-4 h-4 animate-spin" /> : <Zap className="w-4 h-4" />}
                  </Button>
                  <Button variant="ghost" size="icon" className="h-8 w-8" title="Editar" onClick={() => { setEditing(c); setError(null); navigate(`/settings/cameras/${id}/states/edit/${c.id}`) }}>
                    <Pencil className="w-4 h-4" />
                  </Button>
                  <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive" title="Remover" onClick={() => setDeleteId(c.id!)}>
                    <Trash2 className="w-4 h-4" />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </>
      )}

      <ConfirmDialog
        open={deleteId != null}
        title="Remover classificador"
        message="Remover este classificador de estado?"
        confirmLabel="Remover"
        danger
        onConfirm={remove}
        onCancel={() => setDeleteId(null)}
      />
    </SettingsLayout>
  )
}

interface HistoryEntry {
  state: string
  confidence: number
  changed_at: string
  frame: string
  recording_available: boolean
}

// ClassifierHistory mostra as transições de estado de um classificador como grid de
// thumbs (estado + horário). Clicar abre um lightbox com o frame em tamanho cheio; o
// frame é o artefato durável, então vale mesmo quando a gravação já expirou. O botão
// "Ver na gravação" navega para a câmera no instante da transição — habilitado só
// quando ainda há gravação cobrindo aquele momento (recording_available).
function ClassifierHistory({ cameraId, classifier, onBack }: {
  cameraId: string
  classifier: StateClassifier
  onBack: () => void
}) {
  const navigate = useNavigate()
  const [entries, setEntries] = useState<HistoryEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [lightbox, setLightbox] = useState<HistoryEntry | null>(null)

  useEffect(() => {
    let cancelled = false
    fetch(`/api/cameras/${cameraId}/classifiers/${classifier.id}/history?limit=200`, { headers: authHeaders() })
      .then(r => (r.ok ? r.json() : []))
      .then((d: HistoryEntry[]) => { if (!cancelled) setEntries(Array.isArray(d) ? d : []) })
      .catch(() => {})
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [cameraId, classifier.id])

  const frameSrc = (frame: string) => `${frame}?token=${getToken()}`

  return (
    <div id="state-history">
      <div className="mb-4">
        <button id="state-history-back" onClick={onBack} className="text-xs text-muted-foreground hover:text-foreground">← Voltar</button>
        <h3 className="text-sm font-medium text-foreground mt-1">Histórico — {classifier.name}</h3>
      </div>

      {loading ? (
        <p className="text-muted-foreground text-sm">Carregando...</p>
      ) : entries.length === 0 ? (
        <p className="text-muted-foreground text-sm">Nenhuma transição registrada ainda.</p>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-3">
          {entries.map((e, i) => (
            <button
              key={i}
              id={`state-history-thumb-${i}`}
              className="bg-surface border border-border rounded-lg overflow-hidden text-left hover:border-primary/50 transition-colors"
              onClick={() => setLightbox(e)}
            >
              <img src={frameSrc(e.frame)} alt={e.state} className="w-full aspect-video object-cover bg-black" />
              <div className="px-2 py-1.5">
                <p className="text-xs font-medium text-foreground truncate">{e.state}</p>
                <p className="text-[10px] text-muted-foreground">{new Date(e.changed_at).toLocaleString()}</p>
              </div>
            </button>
          ))}
        </div>
      )}

      {lightbox && (
        <div
          id="state-history-lightbox"
          className="fixed inset-0 z-50 bg-black/80 flex items-center justify-center p-4"
          onClick={() => setLightbox(null)}
        >
          <div className="bg-surface rounded-lg overflow-hidden max-w-3xl w-full" onClick={ev => ev.stopPropagation()}>
            <div className="flex items-center justify-between px-4 py-2 border-b border-border">
              <div>
                <p className="text-sm font-medium text-foreground">{lightbox.state}</p>
                <p className="text-[11px] text-muted-foreground">{new Date(lightbox.changed_at).toLocaleString()}</p>
              </div>
              <button id="state-history-lightbox-close" onClick={() => setLightbox(null)} className="text-muted-foreground hover:text-foreground">
                <X className="w-5 h-5" />
              </button>
            </div>
            {lightbox.frame && (
              <img src={frameSrc(lightbox.frame)} alt={lightbox.state} className="w-full max-h-[70vh] object-contain bg-black" />
            )}
            <div className="px-4 py-3 flex items-center justify-end">
              <Button
                id="state-history-watch"
                disabled={!lightbox.recording_available}
                title={lightbox.recording_available ? 'Abrir a gravação neste instante' : 'Gravação expirada'}
                onClick={() => navigate(`/cameras/${cameraId}`, { state: { eventTime: lightbox.changed_at, showRecordings: true } })}
              >
                <Film className="w-3.5 h-3.5" /> {lightbox.recording_available ? 'Ver na gravação' : 'Gravação expirada'}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

interface Sample { crop: string; source: string; frame?: string }
interface ClassRow { label: string; samples: Sample[] }

// RecipientPicker: checkbox de canal (gate) + lista de usuários (Todos/Nenhum)
// que recebem aquele canal. `id` prefixa os ids de teste/automação.
function RecipientPicker({ id, label, enabled, onToggle, users, selected, onSelect }: {
  id: string
  label: string
  enabled: boolean
  onToggle: (v: boolean) => void
  users: { id: number; username: string }[]
  selected: number[]
  onSelect: (ids: number[]) => void
}) {
  const toggleUser = (uid: number, on: boolean) =>
    onSelect(on ? [...selected, uid] : selected.filter(x => x !== uid))
  return (
    <div>
      <label className="flex items-center gap-2 cursor-pointer">
        <input
          id={`recipient-${id}-enabled`}
          type="checkbox"
          className="accent-primary"
          checked={enabled}
          onChange={e => onToggle(e.target.checked)}
        />
        <span className="text-sm text-foreground">{label}</span>
      </label>
      {enabled && (
        <div className="mt-2 border border-border rounded-lg p-2">
          <div className="flex items-center gap-2 mb-1">
            <span className="text-xs text-muted-foreground mr-auto">{selected.length} de {users.length}</span>
            <Button id={`recipient-${id}-all`} variant="ghost" size="sm" onClick={() => onSelect(users.map(u => u.id))}>Todos</Button>
            <Button id={`recipient-${id}-none`} variant="ghost" size="sm" onClick={() => onSelect([])}>Nenhum</Button>
          </div>
          <div className="max-h-40 overflow-y-auto flex flex-col gap-0.5">
            {users.length === 0 && <span className="text-xs text-muted-foreground">Nenhum usuário.</span>}
            {users.map(u => (
              <label key={u.id} className="flex items-center gap-2 cursor-pointer text-sm text-foreground py-0.5">
                <input
                  type="checkbox"
                  className="accent-primary"
                  checked={selected.includes(u.id)}
                  onChange={e => toggleUser(u.id, e.target.checked)}
                />
                {u.username}
              </label>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

export function ClassifierForm({
  cameraId, value, onChange, onDone, onCancel,
}: {
  cameraId: string
  value: StateClassifier
  onChange: (c: StateClassifier) => void
  onDone: () => void
  onCancel: () => void
}) {
  const inputCls = 'w-full bg-surface-2 text-foreground text-sm rounded px-3 py-1.5 border border-border focus:outline-none focus:border-ring'

  // Retângulo do recorte DERIVADO de value.crop_* — sempre reflete o que está
  // salvo/recarregado (sem estado separado que possa divergir). O recorte é
  // obrigatório, então o BboxCanvas roda com deletable={false}: o usuário só
  // redimensiona/posiciona, nunca apaga (handleBox nunca recebe null aqui).
  const box: BboxRect | null = value.crop_w > 0 && value.crop_h > 0 ? cropToBbox(value) : null
  function handleBox(b: BboxRect | null) {
    if (b) onChange({ ...value, ...bboxToCrop(b) })
  }

  const [rows, setRows] = useState<ClassRow[]>(value.classes.map(l => ({ label: l, samples: [] })))
  const [training, setTraining] = useState<string>('')
  const [formError, setFormError] = useState<string>('')
  const [pickerOpen, setPickerOpen] = useState(false)

  // Usuários para os destinatários de notificação/rodapé.
  const [users, setUsers] = useState<{ id: number; username: string }[]>([])
  useEffect(() => {
    fetch('/api/users', { headers: authHeaders() })
      .then(r => r.ok ? r.json() : [])
      .then((us: { id: number; username: string }[]) => setUsers(us))
      .catch(() => {})
  }, [])

  // Quadro principal: ao vivo (snapshot recarregado periodicamente) OU uma imagem
  // estática (evento escolhido / amostra clicada). O retângulo é desenhado sobre ela.
  const [live, setLive] = useState(true)
  const [liveSrc, setLiveSrc] = useState(() => liveSnapshotURL(cameraId))
  const [staticImage, setStaticImage] = useState('')
  // frame do evento atualmente no quadro principal (quando veio do carrossel) —
  // usado para etiquetar a amostra capturada e saber o que já foi inserido.
  const [pickedEventFrame, setPickedEventFrame] = useState('')
  const displaySrc = live ? liveSrc : staticImage

  // Loading do quadro: o container reserva a proporção (aspect-video), então o box
  // já abre no tamanho real; a imagem entra por cima quando carrega. `imgLoaded` é
  // DERIVADO (guarda a última src carregada): ao trocar a fonte, deixa de bater e o
  // spinner reaparece — sem setState em effect.
  const [loadedSrc, setLoadedSrc] = useState('')
  const imgLoaded = displaySrc !== '' && loadedSrc === displaySrc

  // Ao vivo: busca o snapshot como blob e só troca a imagem quando ela já está
  // carregada (objectURL local) — evita o "pulo"/flash do re-fetch a cada tick.
  useEffect(() => {
    if (!live) return
    let cancelled = false
    let prev = ''
    const tick = async () => {
      try {
        const res = await fetch(liveSnapshotURL(cameraId), { headers: authHeaders() })
        if (!res.ok || cancelled) return
        const obj = URL.createObjectURL(await res.blob())
        if (cancelled) { URL.revokeObjectURL(obj); return }
        setLiveSrc(obj)
        if (prev) URL.revokeObjectURL(prev)
        prev = obj
      } catch { /* ignora tick falho */ }
    }
    const iv = setInterval(tick, 2000)
    return () => { cancelled = true; clearInterval(iv); if (prev) URL.revokeObjectURL(prev) }
  }, [live, cameraId])

  function showImage(url: string) { setLive(false); setStaticImage(url) }

  // Reidrata as amostras salvas ao editar um classificador existente.
  useEffect(() => {
    if (value.id == null) return
    fetch(`/api/settings/cameras/${cameraId}/classifiers/${value.id}/samples`, { headers: authHeaders() })
      .then(r => r.ok ? r.json() : { samples: {} })
      .then(async (d: { samples: Record<string, string[]> }) => {
        const loaded: Record<string, Sample[]> = {}
        for (const [label, urls] of Object.entries(d.samples)) {
          loaded[label] = []
          for (const u of urls) {
            // O arquivo salvo é o frame inteiro: source = ele; crop = derivado da região.
            // Quando o nome do arquivo é o frame de origem (_motion.jpg), recupera o
            // vínculo para o carrossel marcar a imagem como já inserida.
            const filename = u.split('/').pop() ?? ''
            const frame = filename.endsWith('_motion.jpg') ? filename : undefined
            const cap = await captureFromUrl(`${u}?token=${getToken()}`, value)
            loaded[label].push({ ...cap, frame })
          }
        }
        setRows(rs => rs.map(r => ({ ...r, samples: loaded[r.label] ?? r.samples })))
      })
      .catch(() => {})
  }, [cameraId, value.id])

  // Ao mover o recorte, re-deriva os crops das amostras (a partir dos frames
  // inteiros) para os thumbs acompanharem a região atual — a caixa é uma só.
  const rowsRef = useRef(rows)
  useEffect(() => { rowsRef.current = rows }, [rows])
  useEffect(() => {
    if (value.crop_w <= 0 || value.crop_h <= 0) return
    let cancelled = false
    const t = setTimeout(async () => {
      const next = await Promise.all(rowsRef.current.map(async r => ({
        ...r,
        samples: await Promise.all(r.samples.map(async s => ({ ...s, crop: (await captureFromUrl(s.source, value)).crop }))),
      })))
      if (!cancelled) setRows(next)
    }, 400)
    return () => { cancelled = true; clearTimeout(t) }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value.crop_x, value.crop_y, value.crop_w, value.crop_h])

  function updateRows(next: ClassRow[]) {
    setRows(next)
    onChange({ ...value, classes: next.map(r => r.label).filter(Boolean) })
  }

  function removeSample(i: number, k: number) {
    updateRows(rows.map((r, j) => j === i ? { ...r, samples: r.samples.filter((_, m) => m !== k) } : r))
  }

  // Captura o recorte do quadro principal como amostra do estado i. Quando o quadro
  // veio de um evento do carrossel, etiqueta a amostra com o frame de origem.
  async function capture(i: number) {
    try {
      const s = await captureFromUrl(live ? liveSnapshotURL(cameraId) : staticImage, value)
      const tagged = { ...s, frame: live ? undefined : (pickedEventFrame || undefined) }
      updateRows(rows.map((r, j) => j === i ? { ...r, samples: [...r.samples, tagged] } : r))
    } catch {
      setTraining('Falha ao capturar o recorte.')
    }
  }

  // Adiciona, de uma vez, várias imagens escolhidas no carrossel como amostras da
  // classe `label` — cada uma recortada pela região atual e etiquetada com o frame.
  async function addEventsToClass(events: EventItem[], label: string) {
    try {
      let anyFallback = false
      const captured = await Promise.all(events.map(async ev => {
        const { url, fellBack } = await loadEventImageURL(cameraId, ev)
        if (fellBack) anyFallback = true
        return { ...(await captureFromUrl(url, value)), frame: ev.frame }
      }))
      // Updater funcional: "Adicionar todos" dispara vários onAddMany em sequência;
      // com rows.map(closure) o segundo sobrescreveria o primeiro. Adicionar amostras
      // não muda as classes, então não precisa do updateRows/onChange aqui.
      setRows(prev => prev.map(r => r.label === label ? { ...r, samples: [...r.samples, ...captured] } : r))
      setTraining(anyFallback
        ? 'Adicionadas. Alguns eventos não têm gravação — usei o snapshot (pode conter a caixa de movimento).'
        : '')
    } catch {
      setTraining('Falha ao adicionar as imagens selecionadas.')
    }
    // Não fecha o carrossel: o usuário pode adicionar outra classificação (rodapé
    // com uma linha por classe). Fechar só pelo botão Fechar/ESC/fundo.
  }

  // Remove do estado `label` a amostra que veio do frame `frame`.
  function removeFromClass(frame: string, label: string) {
    updateRows(rows.map(r => r.label === label
      ? { ...r, samples: r.samples.filter(s => s.frame !== frame) }
      : r))
  }

  // Mapa frame → estado a que pertence (amostras já inseridas nesta sessão).
  const usedByFrame: Record<string, string> = {}
  for (const r of rows) {
    for (const s of r.samples) {
      if (s.frame && !(s.frame in usedByFrame)) usedByFrame[s.frame] = r.label
    }
  }

  // persist salva a config + as amostras (frames inteiros) e devolve o id (ou null em erro).
  async function persist(): Promise<number | null> {
    const err = validateClassifier(value)
    if (err) { setFormError(err); return null }
    setFormError('')
    const isNew = value.id == null
    const res = await fetch(
      isNew ? `/api/settings/cameras/${cameraId}/classifiers` : `/api/settings/cameras/${cameraId}/classifiers/${value.id}`,
      { method: isNew ? 'POST' : 'PUT', headers: { ...authHeaders(), 'Content-Type': 'application/json' }, body: JSON.stringify(value) },
    )
    if (!res.ok) { setFormError((await res.text()).trim() || 'Erro ao salvar'); return null }
    const saved = await res.json()
    const cid: number = saved.id ?? value.id
    // Após criar, devolve o id ao form: os próximos saves viram PUT (mesmo
    // recurso) em vez de criar uma linha duplicada.
    if (isNew && cid != null) onChange({ ...value, id: cid })
    // Persiste o frame INTEIRO (source) — o crop é derivado ao reabrir.
    const samples = rows.flatMap(r => r.samples.map(s => ({ label: r.label, image_b64: s.source, frame: s.frame })))
    await fetch(`/api/settings/cameras/${cameraId}/classifiers/${cid}/samples`, {
      method: 'POST', headers: { ...authHeaders(), 'Content-Type': 'application/json' }, body: JSON.stringify({ samples }),
    })
    return cid
  }

  async function doSave() {
    const cid = await persist()
    if (cid != null) onDone()
  }

  async function saveAndTrain() {
    const cid = await persist()
    if (cid == null) return
    const samples = rows.flatMap(r => r.samples.map(s => ({ label: r.label, image_b64: s.crop })))
    if (new Set(samples.map(s => s.label)).size < 2) { setTraining('Capture imagens de ao menos 2 classes.'); return }
    setTraining('Enviando para treino…')
    const res = await fetch(`/api/settings/cameras/${cameraId}/classifiers/${cid}/train`, {
      method: 'POST', headers: { ...authHeaders(), 'Content-Type': 'application/json' }, body: JSON.stringify({ samples }),
    })
    if (!res.ok) { setTraining((await res.text()).trim() || 'Falha ao treinar'); return }
    const { job_id } = await res.json()
    setTraining(`Treino iniciado (job ${String(job_id).slice(0, 8)}). Será aplicado ao terminar.`)
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        {/* Quadro principal: imagem (ao vivo / evento / amostra) + retângulo do recorte */}
        <div>
          <div className="flex items-center justify-between mb-1">
            <label className="block text-xs text-muted-foreground">Recorte (arraste o retângulo)</label>
            <div className="flex items-center gap-1">
              <Button id="state-frame-live" variant={live ? 'default' : 'ghost'} size="sm" onClick={() => { setLive(true); setPickedEventFrame('') }}>Ao vivo</Button>
              <Button id="state-frame-pick-events" variant="outline" size="sm" onClick={() => setPickerOpen(true)}>Escolher dos eventos</Button>
            </div>
          </div>
          {/* O container reserva a proporção (aspect-video): o box abre no tamanho real
              já com o spinner; a imagem entra por cima ao carregar (sem "pulo"). */}
          <div id="state-frame" className="relative w-full aspect-video rounded overflow-hidden border border-border bg-black">
            <img
              src={displaySrc}
              alt="frame"
              className={`absolute inset-0 w-full h-full object-contain transition-opacity duration-200 ${imgLoaded ? 'opacity-100' : 'opacity-0'}`}
              onLoad={() => setLoadedSrc(displaySrc)}
              onError={e => { (e.currentTarget as HTMLImageElement).style.opacity = '0' }}
            />
            {!imgLoaded && (
              <div id="state-frame-loading" className="absolute inset-0 flex items-center justify-center text-muted-foreground">
                <Loader2 className="w-6 h-6 animate-spin" />
              </div>
            )}
            {/* O recorte só aparece junto com a imagem — escondê-lo enquanto carrega
                evita o retângulo "flutuando" sobre o fundo preto. */}
            <div id="state-frame-overlay" className={`absolute inset-0 transition-opacity duration-200 ${imgLoaded ? 'opacity-100' : 'opacity-0 pointer-events-none'}`}>
              <BboxCanvas box={box} onChange={handleBox} className="w-full h-full" rotatable={false} deletable={false} drawable={false} />
            </div>
          </div>
        </div>

        {/* Configuração do estado + Notificações (cartões lado a lado com o quadro) */}
        <div className="flex flex-col gap-4">
          <div id="state-config-card" className="bg-surface border border-border rounded-lg p-4 flex flex-col gap-3">
            <h3 className="text-sm font-medium text-foreground">Configuração do estado</h3>
            <div>
              <label className="block text-xs text-muted-foreground mb-1">Nome</label>
              <input className={inputCls} value={value.name} onChange={e => onChange({ ...value, name: e.target.value })} />
            </div>
            <div className="grid grid-cols-3 gap-3">
              <div>
                <label className="block text-xs text-muted-foreground mb-1">Intervalo (s)</label>
                <input type="number" min={1} className={inputCls} value={value.trigger_interval_seconds}
                  onChange={e => onChange({ ...value, trigger_interval_seconds: Number(e.target.value) })} />
              </div>
              <div>
                <label className="block text-xs text-muted-foreground mb-1">Limiar (0–1)</label>
                <input type="number" min={0} max={1} step={0.05} className={inputCls} value={value.threshold}
                  onChange={e => onChange({ ...value, threshold: Number(e.target.value) })} />
              </div>
              <div>
                <label className="block text-xs text-muted-foreground mb-1">Confirmações</label>
                <input type="number" min={1} className={inputCls} value={value.min_consecutive}
                  onChange={e => onChange({ ...value, min_consecutive: Number(e.target.value) })} />
              </div>
            </div>
            <label className="flex items-center gap-2 cursor-pointer">
              <input type="checkbox" className="accent-primary" checked={value.enabled}
                onChange={e => onChange({ ...value, enabled: e.target.checked })} />
              <span className="text-sm text-foreground">Ativado</span>
            </label>
          </div>

          {/* Notificação e rodapé: gate + destinatários por usuário (canais independentes) */}
          <div id="state-notify-card" className="bg-surface border border-border rounded-lg p-4 flex flex-col gap-4">
            <h3 className="text-sm font-medium text-foreground">Notificações</h3>
            <RecipientPicker
              id="notify"
              label="Enviar notificação para o usuário"
              enabled={value.notify_enabled ?? false}
              onToggle={v => onChange({ ...value, notify_enabled: v })}
              users={users}
              selected={value.notify_user_ids ?? []}
              onSelect={ids => onChange({ ...value, notify_user_ids: ids })}
            />
            <RecipientPicker
              id="footer"
              label="Exibir no rodapé"
              enabled={value.footer_enabled ?? false}
              onToggle={v => onChange({ ...value, footer_enabled: v })}
              users={users}
              selected={value.footer_user_ids ?? []}
              onSelect={ids => onChange({ ...value, footer_user_ids: ids })}
            />
          </div>
        </div>
      </div>

      {/* Estados cadastrados: um cartão por estado com imagem representativa */}
      <div id="state-classes">
        <div className="flex items-center justify-between mb-2">
          <h3 className="text-sm font-medium text-foreground">Estados cadastrados</h3>
          <Button id="state-class-add" variant="outline" size="sm" onClick={() => updateRows([...rows, { label: '', samples: [] }])}>
            <Plus className="w-3.5 h-3.5" /> Novo estado
          </Button>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          {rows.map((r, i) => (
            <div id={`state-card-${i}`} key={i} className="bg-surface border border-border rounded-lg p-3 flex flex-col gap-3">
              <div className="flex items-center gap-2">
                <span className={`w-2.5 h-2.5 rounded-full shrink-0 ${r.samples.length > 0 ? 'bg-success' : 'bg-muted-foreground/40'}`} />
                <input
                  className={inputCls + ' flex-1'}
                  placeholder="nome do estado (ex: fechado)"
                  value={r.label}
                  onChange={e => updateRows(rows.map((x, j) => j === i ? { ...x, label: e.target.value } : x))}
                />
                <span className="text-xs text-muted-foreground tabular-nums shrink-0">{r.samples.length}</span>
                <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive shrink-0"
                  onClick={() => updateRows(rows.filter((_, j) => j !== i))} title="Remover estado">
                  <Trash2 className="w-4 h-4" />
                </Button>
              </div>
              <div className="flex items-center gap-2 overflow-x-auto py-1 min-h-[5rem]">
                {r.samples.map((s, k) => {
                  // A thumb cuja imagem está no quadro principal fica destacada.
                  const inFrame = !live && staticImage === s.source
                  return (
                  <div key={k} className="relative shrink-0 group/thumb">
                    <img
                      src={s.crop}
                      alt=""
                      onClick={() => { showImage(s.source); setPickedEventFrame(s.frame ?? '') }}
                      title="Ver no quadro principal"
                      aria-current={inFrame ? 'true' : undefined}
                      className={`h-20 w-28 object-cover rounded border cursor-pointer ${inFrame ? 'border-primary ring-2 ring-primary' : 'border-border'}`}
                    />
                    <button
                      type="button"
                      onClick={() => removeSample(i, k)}
                      title="Remover esta imagem"
                      className="absolute -top-1.5 -right-1.5 w-4 h-4 flex items-center justify-center rounded-full bg-destructive text-white text-[10px] leading-none opacity-0 group-hover/thumb:opacity-100 transition-opacity"
                    >
                      ×
                    </button>
                  </div>
                  )
                })}
                {r.samples.length === 0 && <span className="text-xs text-muted-foreground">sem imagem</span>}
              </div>
              <Button variant="outline" size="sm" className="self-start" onClick={() => capture(i)} title="Capturar o recorte do quadro principal">
                <CameraIcon className="w-3.5 h-3.5" /> Capturar imagem
              </Button>
            </div>
          ))}
        </div>
        {training && <p className="text-xs text-muted-foreground mt-2">{training}</p>}
      </div>

      {formError && <p className="text-xs text-red-400">{formError}</p>}
      <div className="flex gap-2">
        <Button id="state-classifier-save" onClick={doSave}>Salvar</Button>
        <Button variant="outline" onClick={onCancel}>Cancelar</Button>
        <Button className="ml-auto" onClick={saveAndTrain} title="Salva as imagens e treina o modelo">
          Salvar e treinar
        </Button>
      </div>

      {pickerOpen && (
        <EventPicker
          cameraId={cameraId}
          classes={rows.map(r => r.label).filter(Boolean)}
          usedByFrame={usedByFrame}
          onPick={ev => { loadEventImageURL(cameraId, ev).then(r => showImage(r.url)); setPickedEventFrame(ev.frame); setPickerOpen(false) }}
          onAddMany={addEventsToClass}
          onRemoveFromClass={(ev, label) => removeFromClass(ev.frame, label)}
          onClose={() => setPickerOpen(false)}
        />
      )}
    </div>
  )
}

function EventPicker({ cameraId, classes, usedByFrame, onPick, onAddMany, onRemoveFromClass, onClose }: {
  cameraId: string
  classes: string[]
  usedByFrame: Record<string, string>
  onPick: (ev: EventItem) => void
  onAddMany: (events: EventItem[], label: string) => void
  onRemoveFromClass: (ev: EventItem, label: string) => void
  onClose: () => void
}) {
  // "Abrir de onde parei": ao escolher uma imagem, guarda o frame + a data dela.
  // Ao reabrir, volta para essa data e centraliza/destaca a imagem escolhida.
  // Sem nada escolhido, abre em hoje e restaura só a rolagem salva.
  const picked0 = loadPicked(cameraId)
  const [date, setDate] = useState(picked0?.date ?? todayStr())
  const [events, setEvents] = useState<EventItem[]>([])
  const [loading, setLoading] = useState(true)
  const [calOpen, setCalOpen] = useState(false)
  const [pickedFrame, setPickedFrame] = useState(picked0?.frame ?? '')
  const [yy, mm, dd] = date.split('-').map(Number)
  const selectedDate = new Date(yy, mm - 1, dd)

  const scrollRef = useRef<HTMLDivElement>(null)
  const selectedRef = useRef<HTMLButtonElement>(null)
  const pendingScrollRef = useRef<number | null>(loadPickerScroll(cameraId)) // fallback = scroll salvo
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Seleção múltipla por checkbox. Cada imagem marcada vai para o BALDE da
  // classificação atual (targetLabel) — assim, trocar o dropdown não move a
  // seleção já feita: cada classificação acumula seu próprio conjunto e ganha
  // uma linha no rodapé. Mapa label → eventos marcados.
  const [buckets, setBuckets] = useState<Record<string, EventItem[]>>({})
  const [targetLabel, setTargetLabel] = useState(classes[0] ?? '')
  const [hideSelected, setHideSelected] = useState(false)
  const bucketOf = (ev: EventItem) =>
    Object.keys(buckets).find(label => buckets[label].some(c => c.frame === ev.frame)) ?? ''
  const isChecked = (ev: EventItem) => bucketOf(ev) !== ''
  // "Selecionado" = já inserido numa classe OU marcado no checkbox.
  const isSelected = (ev: EventItem) => isChecked(ev) || !!usedByFrame[ev.frame]
  // Marca/desmarca: se já está em algum balde, sai dele; senão entra no balde da
  // classificação atual (precisa ter um estado-alvo selecionado).
  function toggleChecked(ev: EventItem) {
    const current = bucketOf(ev)
    if (current) {
      setBuckets(prev => {
        const next = { ...prev, [current]: prev[current].filter(c => c.frame !== ev.frame) }
        if (next[current].length === 0) delete next[current]
        return next
      })
      return
    }
    if (!targetLabel) return
    setBuckets(prev => ({ ...prev, [targetLabel]: [...(prev[targetLabel] ?? []), ev] }))
  }
  // Linhas do rodapé: uma por classificação com seleção pendente.
  const bucketRows = Object.entries(buckets).filter(([, evs]) => evs.length > 0)
  function clearBucket(label: string) {
    setBuckets(prev => { const next = { ...prev }; delete next[label]; return next })
  }
  function addBucket(label: string) {
    onAddMany(buckets[label] ?? [], label)
    clearBucket(label)
  }
  function addAllBuckets() {
    for (const [label, evs] of bucketRows) onAddMany(evs, label)
    setBuckets({})
  }
  const visibleEvents = hideSelected ? events.filter(ev => !isSelected(ev)) : events

  useEffect(() => {
    fetch(`/api/cameras/${cameraId}/motion?date=${date}`, { headers: authHeaders() })
      .then(r => r.ok ? r.json() : { events: [] })
      .then((d: { events?: EventItem[] }) => setEvents((d.events ?? []).filter(e => !!e.frame)))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [cameraId, date])

  // ESC fecha o carrossel.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  // Ao renderizar o carrossel: se a imagem escolhida está nesta data, centraliza
  // ela; senão restaura a rolagem salva (fallback de quando só houve scroll).
  useEffect(() => {
    if (loading || events.length === 0) return
    const container = scrollRef.current
    if (!container) return
    const sel = selectedRef.current
    if (sel) {
      container.scrollLeft = Math.max(0, sel.offsetLeft - (container.clientWidth - sel.clientWidth) / 2)
      pendingScrollRef.current = null
      return
    }
    if (pendingScrollRef.current != null) {
      container.scrollLeft = pendingScrollRef.current
      pendingScrollRef.current = null
    }
  }, [events, loading])

  function changeDate(d: string) {
    pendingScrollRef.current = 0
    setDate(d)
    savePickerScroll(cameraId, 0)
  }

  function onScroll() {
    if (saveTimer.current) clearTimeout(saveTimer.current)
    saveTimer.current = setTimeout(() => {
      savePickerScroll(cameraId, scrollRef.current?.scrollLeft ?? 0)
    }, 300)
  }

  function pick(ev: EventItem) {
    setPickedFrame(ev.frame)
    savePicked(cameraId, { frame: ev.frame, date })
    onPick(ev)
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-6">
      {/* Sem onClick no backdrop: clicar fora NÃO fecha — só o botão Fechar ou ESC,
          para não perder a seleção em lote por um clique acidental. */}
      <div className="bg-surface border border-border rounded-lg p-4 max-w-5xl w-full">
        <div className="flex items-center justify-between mb-3 gap-3">
          <h3 className="text-sm font-medium text-foreground flex items-center gap-2 flex-wrap">
            <span>Escolha imagem dos eventos para</span>
            <select
              id="event-picker-target"
              className="bg-surface-2 text-foreground text-sm rounded px-2 py-1 border border-border focus:outline-none focus:border-ring"
              value={targetLabel}
              onChange={e => setTargetLabel(e.target.value)}
            >
              {classes.length === 0 && <option value="">(defina um estado)</option>}
              {classes.map(c => <option key={c} value={c}>{c}</option>)}
            </select>
          </h3>
          <div className="flex items-center gap-2 shrink-0">
            {/* Mesmo calendário (DayPicker + ptBR) da lista de gravações, em popover. */}
            <div className="relative">
              <Button variant="outline" size="sm" className="tabular-nums" onClick={() => setCalOpen(o => !o)}>
                {format(selectedDate, 'dd/MM/yyyy', { locale: ptBR })}
              </Button>
              {calOpen && (
                <div className="absolute right-0 mt-1 z-10 bg-surface border border-border rounded-lg p-2 shadow-xl">
                  <DayPicker
                    mode="single"
                    selected={selectedDate}
                    defaultMonth={selectedDate}
                    disabled={{ after: new Date() }}
                    onSelect={d => { if (d) { changeDate(format(d, 'yyyy-MM-dd')); setCalOpen(false) } }}
                    locale={ptBR}
                  />
                </div>
              )}
            </div>
            <Button variant="ghost" size="sm" onClick={onClose}>Fechar</Button>
          </div>
        </div>

        <label className="flex items-center gap-2 text-xs text-muted-foreground mb-2 cursor-pointer select-none w-fit">
          <input
            type="checkbox"
            className="accent-primary"
            checked={hideSelected}
            onChange={e => setHideSelected(e.target.checked)}
          />
          Não exibir selecionados
        </label>

        {loading ? (
          <p className="text-sm text-muted-foreground">Carregando eventos…</p>
        ) : events.length === 0 ? (
          <p className="text-sm text-muted-foreground">Nenhum evento com snapshot hoje.</p>
        ) : visibleEvents.length === 0 ? (
          <p className="text-sm text-muted-foreground">Todos os eventos desta data já foram selecionados.</p>
        ) : (
          <div ref={scrollRef} onScroll={onScroll} className="flex gap-2 overflow-x-auto pb-2">
            {visibleEvents.map((ev, i) => {
              const selected = ev.frame === pickedFrame
              const usedLabel = usedByFrame[ev.frame]
              const bucket = bucketOf(ev)
              const highlighted = !!usedLabel || !!bucket || selected
              return (
                <div key={i} className="relative shrink-0">
                  {usedLabel ? (
                    // Já inserida: checada (indicador) + badge do estado + X que remove.
                    <input
                      type="checkbox"
                      checked
                      readOnly
                      title={`Já adicionada em "${usedLabel}"`}
                      className="absolute top-1.5 left-1.5 z-10 w-5 h-5 accent-primary"
                    />
                  ) : (
                    <input
                      type="checkbox"
                      checked={isChecked(ev)}
                      onChange={() => toggleChecked(ev)}
                      onClick={e => e.stopPropagation()}
                      title="Selecionar para adicionar em lote"
                      className="absolute top-1.5 left-1.5 z-10 w-5 h-5 accent-primary cursor-pointer"
                    />
                  )}

                  {usedLabel && (
                    <div className="absolute top-1.5 right-1.5 z-10 flex items-center gap-1">
                      <span className="px-1.5 py-0.5 rounded bg-primary text-on-primary text-[11px] leading-none max-w-[8rem] truncate">
                        {usedLabel}
                      </span>
                      <button
                        type="button"
                        onClick={e => { e.stopPropagation(); onRemoveFromClass(ev, usedLabel) }}
                        title={`Remover de "${usedLabel}"`}
                        className="w-5 h-5 flex items-center justify-center rounded-full bg-destructive text-white text-xs leading-none"
                      >
                        ×
                      </button>
                    </div>
                  )}

                  {!usedLabel && bucket && (
                    // Marcada (ainda não inserida): badge da classificação de destino.
                    <span className="absolute top-1.5 right-1.5 z-10 px-1.5 py-0.5 rounded bg-surface-2 text-foreground border border-primary/50 text-[11px] leading-none max-w-[8rem] truncate">
                      {bucket}
                    </span>
                  )}

                  <button
                    ref={selected ? selectedRef : undefined}
                    id={selected ? 'event-picker-selected' : undefined}
                    aria-current={selected ? 'true' : undefined}
                    onClick={() => pick(ev)}
                    title={new Date(ev.time).toLocaleString('pt-BR')}
                    className={`block rounded overflow-hidden border transition-colors ${
                      highlighted ? 'border-primary ring-2 ring-primary' : 'border-border hover:border-primary'
                    }`}
                  >
                    <img src={eventSnapshotURL(cameraId, ev)} alt="" className="h-56 w-auto block" />
                  </button>
                </div>
              )
            })}
          </div>
        )}

        {bucketRows.length > 0 && (
          <div className="mt-3 border-t border-border pt-3 flex flex-col gap-2">
            {/* Uma linha por classificação com seleção pendente. */}
            {bucketRows.map(([label, evs]) => (
              <div key={label} id={`event-picker-row-${label}`} className="flex items-center gap-2">
                <span className="text-sm text-foreground mr-auto">
                  <span className="tabular-nums">{evs.length}</span> selecionada(s) em “{label}”
                </span>
                <Button variant="ghost" size="sm" onClick={() => clearBucket(label)}>Limpar</Button>
                <Button size="sm" onClick={() => addBucket(label)}>Adicionar {evs.length}</Button>
              </div>
            ))}

            {/* Linha "todos" só quando há mais de uma classificação. */}
            {bucketRows.length > 1 && (
              <div id="event-picker-row-all" className="flex items-center gap-2 border-t border-border pt-2">
                <span className="text-sm text-muted-foreground mr-auto">
                  {bucketRows.length} classificações
                </span>
                <Button variant="ghost" size="sm" onClick={() => setBuckets({})}>Limpar todos</Button>
                <Button size="sm" onClick={addAllBuckets}>Adicionar todos</Button>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
