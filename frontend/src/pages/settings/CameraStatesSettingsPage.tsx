import { useState, useEffect, useCallback, useRef } from 'react'
import { useParams } from 'react-router-dom'
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
import { Plus, Pencil, Trash2, Camera as CameraIcon } from '../../components/Icons'
import {
  type StateClassifier,
  bboxToCrop,
  cropToBbox,
  validateClassifier,
} from './stateClassifier'

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

export default function CameraStatesSettingsPage() {
  const { id } = useParams<{ id: string }>()
  const isAdmin = getRole() === 'admin'
  const [items, setItems] = useState<StateClassifier[]>([])
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<StateClassifier | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [deleteId, setDeleteId] = useState<number | null>(null)
  const [states, setStates] = useState<Record<number, string>>({})

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
      .then((data: StateClassifier[]) => setItems(data))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [id])

  async function remove() {
    if (deleteId == null) return
    await fetch(`/api/settings/cameras/${id}/classifiers/${deleteId}`, { method: 'DELETE', headers: authHeaders() })
    setDeleteId(null)
    await reload()
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
          onDone={() => { setEditing(null); reload() }}
          onCancel={() => { setEditing(null); setError(null) }}
        />
      ) : (
        <>
          <div className="flex items-center justify-between mb-4">
            <p className="text-sm text-muted-foreground">Classificadores de estado (recorte fixo → estado).</p>
            <Button id="state-classifier-new" onClick={() => { setEditing(emptyClassifier()); setError(null) }}>
              <Plus className="w-3.5 h-3.5" /> Novo classificador
            </Button>
          </div>

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
                  <Button variant="ghost" size="icon" className="h-8 w-8" title="Editar" onClick={() => { setEditing(c); setError(null) }}>
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

interface Sample { crop: string; source: string }
interface ClassRow { label: string; samples: Sample[] }

function ClassifierForm({
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
  // salvo/recarregado (sem estado separado que possa divergir). w/h=0 = sem recorte
  // (a lixeira do BboxCanvas zera; o save reclama até redesenhar).
  const box: BboxRect | null = value.crop_w > 0 && value.crop_h > 0 ? cropToBbox(value) : null
  function handleBox(b: BboxRect | null) {
    if (b) onChange({ ...value, ...bboxToCrop(b) })
    else onChange({ ...value, crop_w: 0, crop_h: 0 })
  }

  const [rows, setRows] = useState<ClassRow[]>(value.classes.map(l => ({ label: l, samples: [] })))
  const [training, setTraining] = useState<string>('')
  const [formError, setFormError] = useState<string>('')
  const [pickerOpen, setPickerOpen] = useState(false)
  const [pickerDate, setPickerDate] = useState<string>(todayStr())

  // Quadro principal: ao vivo (snapshot recarregado periodicamente) OU uma imagem
  // estática (evento escolhido / amostra clicada). O retângulo é desenhado sobre ela.
  const [live, setLive] = useState(true)
  const [liveSrc, setLiveSrc] = useState(() => liveSnapshotURL(cameraId))
  const [staticImage, setStaticImage] = useState('')
  const displaySrc = live ? liveSrc : staticImage

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
            loaded[label].push(await captureFromUrl(`${u}?token=${getToken()}`, value))
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

  // Captura o recorte do quadro principal como amostra do estado i.
  async function capture(i: number) {
    try {
      const s = await captureFromUrl(live ? liveSnapshotURL(cameraId) : staticImage, value)
      updateRows(rows.map((r, j) => j === i ? { ...r, samples: [...r.samples, s] } : r))
    } catch {
      setTraining('Falha ao capturar o recorte.')
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
    // Persiste o frame INTEIRO (source) — o crop é derivado ao reabrir.
    const samples = rows.flatMap(r => r.samples.map(s => ({ label: r.label, image_b64: s.source })))
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
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Quadro principal: imagem (ao vivo / evento / amostra) + retângulo do recorte */}
        <div>
          <div className="flex items-center justify-between mb-1">
            <label className="block text-xs text-muted-foreground">Recorte (arraste o retângulo)</label>
            <div className="flex items-center gap-1">
              <Button variant={live ? 'default' : 'ghost'} size="sm" onClick={() => setLive(true)}>Ao vivo</Button>
              <Button variant="outline" size="sm" onClick={() => setPickerOpen(true)}>Escolher dos eventos</Button>
            </div>
          </div>
          <div className="relative w-full rounded overflow-hidden border border-border bg-black">
            <img
              src={displaySrc}
              alt="frame"
              className="w-full block"
              onError={e => { (e.currentTarget as HTMLImageElement).style.opacity = '0' }}
            />
            <div className="absolute inset-0">
              <BboxCanvas box={box} onChange={handleBox} className="w-full h-full" />
            </div>
          </div>
        </div>

        {/* Campos */}
        <div className="flex flex-col gap-3">
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
      </div>

      {/* Estados (classes) com imagem representativa */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <label className="block text-xs text-muted-foreground">Estados — capture o recorte que representa cada um</label>
          <Button variant="outline" size="sm" onClick={() => updateRows([...rows, { label: '', samples: [] }])}>
            <Plus className="w-3.5 h-3.5" /> Estado
          </Button>
        </div>
        <div className="flex flex-col gap-2">
          {rows.map((r, i) => (
            <div key={i} className="flex items-center gap-3 bg-surface border border-border rounded-lg px-3 py-4">
              <input
                className={inputCls + ' max-w-[12rem]'}
                placeholder="nome do estado (ex: fechado)"
                value={r.label}
                onChange={e => updateRows(rows.map((x, j) => j === i ? { ...x, label: e.target.value } : x))}
              />
              <div className="flex items-center gap-2 flex-1 overflow-x-auto py-1">
                {r.samples.map((s, k) => (
                  <div key={k} className="relative shrink-0 group/thumb">
                    <img
                      src={s.crop}
                      alt=""
                      onClick={() => showImage(s.source)}
                      title="Ver no quadro principal"
                      className="h-20 w-28 object-cover rounded border border-border cursor-pointer"
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
                ))}
                {r.samples.length === 0 && <span className="text-xs text-muted-foreground">sem imagem</span>}
              </div>
              <span className="text-xs text-muted-foreground tabular-nums">{r.samples.length}</span>
              <Button variant="outline" size="sm" onClick={() => capture(i)} title="Capturar o recorte do quadro principal">
                <CameraIcon className="w-3.5 h-3.5" /> Capturar
              </Button>
              <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive"
                onClick={() => updateRows(rows.filter((_, j) => j !== i))} title="Remover estado">
                <Trash2 className="w-4 h-4" />
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
          date={pickerDate}
          onDateChange={setPickerDate}
          onPick={ev => { showImage(eventFrameURL(cameraId, ev)); setPickerOpen(false) }}
          onClose={() => setPickerOpen(false)}
        />
      )}
    </div>
  )
}

function EventPicker({ cameraId, date, onDateChange, onPick, onClose }: {
  cameraId: string
  date: string
  onDateChange: (d: string) => void
  onPick: (ev: EventItem) => void
  onClose: () => void
}) {
  const [events, setEvents] = useState<EventItem[]>([])
  const [loading, setLoading] = useState(true)
  const [calOpen, setCalOpen] = useState(false)
  const [yy, mm, dd] = date.split('-').map(Number)
  const selectedDate = new Date(yy, mm - 1, dd)

  useEffect(() => {
    fetch(`/api/cameras/${cameraId}/motion?date=${date}`, { headers: authHeaders() })
      .then(r => r.ok ? r.json() : { events: [] })
      .then((d: { events?: EventItem[] }) => setEvents((d.events ?? []).filter(e => !!e.frame)))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [cameraId, date])

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-6" onClick={onClose}>
      <div className="bg-surface border border-border rounded-lg p-4 max-w-5xl w-full" onClick={e => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-3 gap-3">
          <h3 className="text-sm font-medium text-foreground">Escolher imagem dos eventos — clique no frame que representa o estado</h3>
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
                    onSelect={d => { if (d) { onDateChange(format(d, 'yyyy-MM-dd')); setCalOpen(false) } }}
                    locale={ptBR}
                  />
                </div>
              )}
            </div>
            <Button variant="ghost" size="sm" onClick={onClose}>Fechar</Button>
          </div>
        </div>
        {loading ? (
          <p className="text-sm text-muted-foreground">Carregando eventos…</p>
        ) : events.length === 0 ? (
          <p className="text-sm text-muted-foreground">Nenhum evento com snapshot hoje.</p>
        ) : (
          <div className="flex gap-2 overflow-x-auto pb-2">
            {events.map((ev, i) => (
              <button
                key={i}
                onClick={() => onPick(ev)}
                title={new Date(ev.time).toLocaleString('pt-BR')}
                className="shrink-0 rounded overflow-hidden border border-border hover:border-primary transition-colors"
              >
                <img src={eventSnapshotURL(cameraId, ev)} alt="" className="h-56 w-auto block" />
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
