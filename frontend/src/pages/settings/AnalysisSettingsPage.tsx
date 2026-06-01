import { useEffect, useRef, useState } from 'react'
import SettingsLayout from '../../components/SettingsLayout'
import BboxCanvas, { type BboxRect } from '../../components/BboxCanvas'
import ConfirmDialog from '../../components/ConfirmDialog'
import { useSettings, type CameraSettings } from '../../hooks/useSettings'
import { authHeaders, getToken } from '../../auth'

interface AnalysisConfig {
  enabled: boolean
  service_url: string
  model: string
  confidence_threshold: number
  has_custom_model?: boolean
}

const MODELS = [
  { group: 'YOLOv8',  names: ['yolov8n', 'yolov8s', 'yolov8m', 'yolov8l', 'yolov8x'] },
  { group: 'YOLO11',  names: ['yolo11n', 'yolo11s', 'yolo11m', 'yolo11l', 'yolo11x'] },
  { group: 'YOLO12',  names: ['yolo12n', 'yolo12s', 'yolo12m', 'yolo12l', 'yolo12x'] },
]

interface EventItem {
  id: number
  time: string
  score: number
  frame?: string
  label?: string
}

function frameURL(cameraId: string, eventTime: string, frame: string): string {
  const d = new Date(eventTime)
  const dateDir = `${d.getUTCFullYear()}/${String(d.getUTCMonth() + 1).padStart(2, '0')}/${String(d.getUTCDate()).padStart(2, '0')}`
  return `/recordings/${cameraId}/${dateDir}/${frame}?token=${getToken()}`
}

const LIMIT = 20

export default function AnalysisSettingsPage() {
  const { settings } = useSettings()
  const cameras: CameraSettings[] = settings?.cameras ?? []
  const [cfg, setCfg] = useState<AnalysisConfig>({
    enabled: false,
    service_url: '',
    model: 'yolov8n',
    confidence_threshold: 0.4,
  })
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')
  // label section state — null means "not yet loaded / loading"
  const [labelCamID, setLabelCamID] = useState('')
  const [unlabeledOnly, setUnlabeledOnly] = useState(true)
  const [labelSearch, setLabelSearch] = useState('')
  const [labelPage, setLabelPage] = useState(1)
  const [labelEvents, setLabelEvents] = useState<EventItem[] | null>(null)
  const [labelTotal, setLabelTotal] = useState(0)
  const [labelInputs, setLabelInputs] = useState<Record<number, string>>({})
  const [labelSaveState, setLabelSaveState] = useState<Record<number, 'saved' | 'error'>>({})
  const [zoomEvent, setZoomEvent] = useState<{ src: string; id: number } | null>(null)
  const [labelRefreshTick, setLabelRefreshTick] = useState(0)
  const labelLoading = labelCamID !== '' && labelEvents === null

  // bulk selection state
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [bulkLabel, setBulkLabel] = useState('')
  const [bulkBusy, setBulkBusy] = useState(false)
  const [bulkConfirm, setBulkConfirm] = useState<null | { action: 'delete' | 'label'; label?: string }>(null)
  const [bulkError, setBulkError] = useState('')

  // per-row inline delete state
  const [rowDeleteConfirm, setRowDeleteConfirm] = useState<EventItem | null>(null)
  const [rowDeleteBusy, setRowDeleteBusy] = useState(false)

  function toggleSelect(id: number) {
    setSelected(s => {
      const n = new Set(s)
      if (n.has(id)) n.delete(id); else n.add(id)
      return n
    })
  }
  function selectAllOnPage() {
    setSelected(new Set((labelEvents ?? []).map(e => e.id)))
  }
  function clearSelection() {
    setSelected(new Set())
    setBulkLabel('')
    setBulkError('')
  }
  async function executeBulkDelete() {
    const ids = Array.from(selected)
    setBulkBusy(true)
    setBulkError('')
    try {
      const r = await fetch('/api/events/bulk', {
        method: 'DELETE',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ ids }),
      })
      if (!r.ok) { setBulkError('Erro ao excluir'); return }
      clearSelection()
      setBulkConfirm(null)
      // refresh page; back off if it became empty
      const newTotal = labelTotal - ids.length
      const lastPage = Math.max(1, Math.ceil(newTotal / LIMIT))
      if (labelPage > lastPage) {
        setLabelPage(lastPage)
      }
      setLabelEvents(null)
      setLabelRefreshTick(t => t + 1)
      refreshCounts()
    } finally {
      setBulkBusy(false)
    }
  }
  async function executeRowDelete() {
    if (!rowDeleteConfirm) return
    const id = rowDeleteConfirm.id
    setRowDeleteBusy(true)
    try {
      const r = await fetch('/api/events/bulk', {
        method: 'DELETE',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ ids: [id] }),
      })
      if (!r.ok) return
      setRowDeleteConfirm(null)
      const newTotal = labelTotal - 1
      const lastPage = Math.max(1, Math.ceil(newTotal / LIMIT))
      if (labelPage > lastPage) {
        setLabelPage(lastPage)
      }
      setLabelEvents(null)
      setLabelRefreshTick(t => t + 1)
      refreshCounts()
    } finally {
      setRowDeleteBusy(false)
    }
  }
  async function executeBulkLabel() {
    const ids = Array.from(selected)
    const label = bulkLabel
    setBulkBusy(true)
    setBulkError('')
    try {
      const r = await fetch('/api/events/bulk/label', {
        method: 'PATCH',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ ids, label }),
      })
      if (!r.ok) { setBulkError('Erro ao aplicar label'); return }
      clearSelection()
      setBulkConfirm(null)
      setLabelEvents(null)
      setLabelRefreshTick(t => t + 1)
      refreshCounts()
    } finally {
      setBulkBusy(false)
    }
  }

  // bbox drawing state for zoom modal
  const [annBox, setAnnBox] = useState<BboxRect | null>(null)
  const [annLabel, setAnnLabel] = useState('')
  const [annSaving, setAnnSaving] = useState(false)
  const [annSaveOk, setAnnSaveOk] = useState(false)
  const [existingAnn, setExistingAnn] = useState<BboxRect | null>(null)
  const [existingAnnId, setExistingAnnId] = useState<number | null>(null)
  const [existingAnnLabel, setExistingAnnLabel] = useState('')

  const [annCount, setAnnCount] = useState<number | null>(null)
  const [labelCount, setLabelCount] = useState<number | null>(null)
  const [epochs, setEpochs] = useState(20)
  const [ftJobID, setFtJobID] = useState<string | null>(() => localStorage.getItem('ft_job_id'))
  const [ftStatus, setFtStatus] = useState<{ status: string; epoch: number; total_epochs: number; error: string } | null>(null)
  const [ftError, setFtError] = useState('')
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    fetch('/api/settings/analysis', { headers: authHeaders() })
      .then(r => r.json())
      .then(setCfg)
      .catch(() => setError('Falha ao carregar configurações'))
    refreshCounts()
  }, [])

  function refreshCounts() {
    fetch('/api/settings/analysis/annotation-count', { headers: authHeaders() })
      .then(r => r.ok ? r.json() : null)
      .then(d => {
        if (d) {
          setAnnCount(d.count ?? 0)
          setLabelCount(d.label_count ?? 0)
        }
      })
      .catch(() => {})
  }

  useEffect(() => {
    if (!zoomEvent) return
    function onKey(e: KeyboardEvent) { if (e.key === 'Escape') closeZoomModal() }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [zoomEvent])

  useEffect(() => {
    if (!zoomEvent) return
    fetch(`/api/events/${zoomEvent.id}/annotations`, { headers: authHeaders() })
      .then(r => r.ok ? r.json() : [])
      .then((list: Array<{ id: number; label: string; bbox_x: number; bbox_y: number; bbox_w: number; bbox_h: number; rotation_deg?: number }>) => {
        const a = list[0]
        setExistingAnnId(a?.id ?? null)
        setExistingAnnLabel(a?.label ?? '')
        setExistingAnn(a ? {
          x: a.bbox_x - a.bbox_w / 2,
          y: a.bbox_y - a.bbox_h / 2,
          w: a.bbox_w,
          h: a.bbox_h,
          rotation_deg: a.rotation_deg ?? 0,
        } : null)
        // annotation label takes priority over event label
        if (a?.label) setAnnLabel(a.label)
      })
      .catch(() => {})
  }, [zoomEvent])

  function openZoomModal(src: string, id: number, eventLabel = '') {
    setAnnBox(null)
    setAnnLabel(eventLabel)
    setAnnSaveOk(false)
    setExistingAnn(null)
    setExistingAnnId(null)
    setExistingAnnLabel('')
    setZoomEvent({ src, id })
  }

  function closeZoomModal() {
    setZoomEvent(null)
    setAnnBox(null)
    setAnnLabel('')
    setAnnSaveOk(false)
    setExistingAnn(null)
    setExistingAnnId(null)
    setExistingAnnLabel('')
  }

  function handleAnnBoxChange(box: BboxRect | null) {
    setAnnBox(box)
    setAnnSaveOk(false)
  }

  async function deleteAnnotation() {
    if (!existingAnnId) return
    const r = await fetch(`/api/annotations/${existingAnnId}`, {
      method: 'DELETE',
      headers: authHeaders(),
    })
    if (r.ok) {
      setExistingAnn(null)
      setExistingAnnId(null)
      setExistingAnnLabel('')
      setAnnLabel('')
      refreshCounts()
    }
  }

  async function saveAnnotation() {
    if (!annBox || !zoomEvent || annBox.w < 0.01 || annBox.h < 0.01) return
    setAnnSaving(true)
    try {
      const res = await fetch(`/api/events/${zoomEvent.id}/annotations`, {
        method: 'POST',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify({
          label: annLabel,
          bbox_x: annBox.x + annBox.w / 2,
          bbox_y: annBox.y + annBox.h / 2,
          bbox_w: annBox.w,
          bbox_h: annBox.h,
          rotation_deg: annBox.rotation_deg ?? 0,
        }),
      })
      if (res.ok) {
        // sync event label with annotation label if they differ
        const currentEventLabel = labelInputs[zoomEvent.id] ?? ''
        if (annLabel !== currentEventLabel) {
          await fetch(`/api/events/${zoomEvent.id}/label`, {
            method: 'PATCH',
            headers: { ...authHeaders(), 'Content-Type': 'application/json' },
            body: JSON.stringify({ label: annLabel }),
          })
          setLabelInputs(s => ({ ...s, [zoomEvent.id]: annLabel }))
          setLabelEvents(prev => prev?.map(e => e.id === zoomEvent.id ? { ...e, label: annLabel || undefined } : e) ?? null)
        }
        setExistingAnn({ ...annBox })
        setExistingAnnLabel(annLabel)
        setAnnBox(null)
        setAnnLabel(annLabel)
        setAnnSaveOk(true)
        refreshCounts()
        setTimeout(() => setAnnSaveOk(false), 1500)
      }
    } finally {
      setAnnSaving(false)
    }
  }

  useEffect(() => {
    if (!ftJobID) return
    // Fetch once immediately to restore state when returning to the page
    fetch(`/api/settings/analysis/finetune/status/${ftJobID}`, { headers: authHeaders() })
      .then(r => r.ok ? r.json() : null)
      .then(s => { if (s) setFtStatus(s) })
      .catch(() => {})
    pollRef.current = setInterval(async () => {
      try {
        const r = await fetch(`/api/settings/analysis/finetune/status/${ftJobID}`, { headers: authHeaders() })
        if (!r.ok) return
        const s = await r.json()
        setFtStatus(s)
        if (s.status === 'done' || s.status === 'error') {
          clearInterval(pollRef.current!)
          pollRef.current = null
          localStorage.removeItem('ft_job_id')
          if (s.status === 'error') setFtError(s.error || 'Erro no treino')
          if (s.status === 'done') {
            fetch('/api/settings/analysis', { headers: authHeaders() })
              .then(r => r.json())
              .then(data => setCfg(data))
              .catch(() => {})
          }
        }
      } catch { /* ignore poll errors */ }
    }, 3000)
    return () => { if (pollRef.current) clearInterval(pollRef.current) }
  }, [ftJobID])

  useEffect(() => {
    if (!labelCamID) return
    const controller = new AbortController()
    const params = new URLSearchParams({
      page: String(labelPage),
      limit: String(LIMIT),
      ...(unlabeledOnly && !labelSearch ? { unlabeled: 'true' } : {}),
      ...(labelSearch ? { label: labelSearch } : {}),
    })
    fetch(`/api/cameras/${labelCamID}/events?${params}`, {
      headers: authHeaders(),
      signal: controller.signal,
    })
      .then(r => r.json())
      .then(d => {
        setLabelEvents(d.events ?? [])
        setLabelTotal(d.total ?? 0)
        const inputs: Record<number, string> = {}
        for (const ev of (d.events ?? [])) inputs[ev.id] = ev.label ?? ''
        setLabelInputs(inputs)
        setLabelSaveState({})
      })
      .catch(err => { if (err.name !== 'AbortError') setLabelEvents([]) })
    return () => controller.abort()
  }, [labelCamID, unlabeledOnly, labelSearch, labelPage, labelRefreshTick])

  function handleLabelBlur(eventId: number) {
    const label = labelInputs[eventId] ?? ''
    fetch(`/api/events/${eventId}/label`, {
      method: 'PATCH',
      headers: { ...authHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify({ label }),
    }).then(r => {
      setLabelSaveState(s => ({ ...s, [eventId]: r.ok ? 'saved' : 'error' }))
      if (r.ok) {
        refreshCounts()
        setTimeout(() => setLabelSaveState(s => { const n = { ...s }; delete n[eventId]; return n }), 1200)
      }
    }).catch(() => setLabelSaveState(s => ({ ...s, [eventId]: 'error' })))
  }

  async function handleStartFinetune() {
    setFtError('')
    setFtStatus(null)
    setFtJobID(null)
    localStorage.removeItem('ft_job_id')
    const res = await fetch('/api/settings/analysis/finetune', {
      method: 'POST',
      headers: { ...authHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify({ epochs }),
    })
    if (!res.ok) {
      const msg = await res.text()
      setFtError(msg || 'Erro ao iniciar treino')
      return
    }
    const { job_id } = await res.json()
    localStorage.setItem('ft_job_id', job_id)
    setFtJobID(job_id)
    setFtStatus({ status: 'pending', epoch: 0, total_epochs: 20, error: '' })
  }

  async function handleCancelFinetune() {
    if (!ftJobID) return
    await fetch(`/api/settings/analysis/finetune/${ftJobID}`, {
      method: 'DELETE',
      headers: authHeaders(),
    })
    if (pollRef.current) { clearInterval(pollRef.current); pollRef.current = null }
    localStorage.removeItem('ft_job_id')
    setFtJobID(null)
    setFtStatus(null)
    setFtError('')
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setSaving(true)
    try {
      const res = await fetch('/api/settings/analysis', {
        method: 'PUT',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify(cfg),
      })
      if (res.ok) {
        setSaved(true)
        setTimeout(() => setSaved(false), 2000)
      } else {
        setError('Erro ao salvar')
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <SettingsLayout>
      <div className="space-y-6">
        <div>
          <h3 className="text-lg font-semibold text-white mb-1">Análise de vídeo</h3>
          <p className="text-sm text-gray-400">
            Serviço YOLO para detecção de objetos em gravações. Cada chunk MP4 é analisado após ser fechado.
          </p>
        </div>

        <form onSubmit={handleSave} className="bg-gray-800 rounded-lg border border-gray-700 divide-y divide-gray-700">
          <div className="p-4 flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-gray-200">Ativar análise</p>
              <p className="text-xs text-gray-500 mt-0.5">Habilita o envio de gravações para o serviço YOLO</p>
            </div>
            <button
              type="button"
              onClick={() => setCfg(c => ({ ...c, enabled: !c.enabled }))}
              className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${cfg.enabled ? 'bg-blue-600' : 'bg-gray-600'}`}
            >
              <span className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${cfg.enabled ? 'translate-x-6' : 'translate-x-1'}`} />
            </button>
          </div>

          <div className="p-4 space-y-4">
            <div>
              <label className="block text-xs font-medium text-gray-400 mb-1">URL do serviço</label>
              <input
                type="url"
                className="w-full bg-gray-700 text-gray-200 text-sm rounded px-3 py-2 border border-gray-600 focus:outline-none focus:border-blue-500"
                placeholder="http://yolo:8001"
                value={cfg.service_url}
                onChange={e => setCfg(c => ({ ...c, service_url: e.target.value }))}
              />
              <p className="text-xs text-gray-500 mt-1">Endereço do container YOLO (ex: <code>http://yolo:8001</code>)</p>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-medium text-gray-400 mb-1">Modelo</label>
                <select
                  className="w-full bg-gray-700 text-gray-200 text-sm rounded px-3 py-2 border border-gray-600 focus:outline-none focus:border-blue-500"
                  value={cfg.model}
                  onChange={e => setCfg(c => ({ ...c, model: e.target.value }))}
                >
                  {cfg.has_custom_model && (
                    <optgroup label="Custom">
                      <option value="custom">custom ✓ (treinado)</option>
                      <option value="custom+yolov8n">custom + yolov8n</option>
                    </optgroup>
                  )}
                  {MODELS.map(({ group, names }) => (
                    <optgroup key={group} label={group}>
                      {names.map(m => (
                        <option key={m} value={m}>{m}</option>
                      ))}
                    </optgroup>
                  ))}
                </select>
                <p className="text-xs text-gray-500 mt-1">n = mais rápido · x = mais preciso</p>
              </div>

              <div>
                <label className="block text-xs font-medium text-gray-400 mb-1">
                  Limiar de confiança ({(cfg.confidence_threshold * 100).toFixed(0)}%)
                </label>
                <input
                  type="range"
                  min={0.1}
                  max={0.9}
                  step={0.05}
                  className="w-full accent-blue-500"
                  value={cfg.confidence_threshold}
                  onChange={e => setCfg(c => ({ ...c, confidence_threshold: Number(e.target.value) }))}
                />
                <div className="flex justify-between text-xs text-gray-500 mt-0.5">
                  <span>10%</span><span>90%</span>
                </div>
              </div>
            </div>
          </div>

          <div className="p-4 flex items-center justify-between">
            {error && <p className="text-sm text-red-400">{error}</p>}
            {saved && <p className="text-sm text-green-400">Salvo</p>}
            {!error && !saved && <span />}
            <button
              type="submit"
              disabled={saving}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white text-sm rounded transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
            >
              {saving ? 'Salvando...' : 'Salvar'}
            </button>
          </div>
        </form>

        <div className="bg-gray-800 rounded-lg border border-gray-700 p-4">
          <h4 className="text-sm font-medium text-gray-200 mb-2">Como usar</h4>
          <ol className="text-xs text-gray-400 space-y-1 list-decimal list-inside">
            <li>Suba o serviço YOLO: <code className="bg-gray-700 px-1 rounded">docker compose --profile yolo up -d</code></li>
            <li>Configure a URL acima (padrão: <code className="bg-gray-700 px-1 rounded">http://yolo:8001</code>)</li>
            <li>Ative a análise global e, se necessário, por câmera em Configurações → Câmeras → Análise</li>
            <li>Na próxima limpeza do storage, as gravações concluídas serão analisadas automaticamente</li>
          </ol>
        </div>

        <div className="bg-gray-800 rounded-lg border border-gray-700 divide-y divide-gray-700">
          <div className="p-4">
            <h4 className="text-sm font-semibold text-gray-200 mb-1">Fine-tuning</h4>
            <p className="text-xs text-gray-400">
              Treina um modelo personalizado usando os snapshots que você anotou nos eventos de movimento.
              O modelo gerado (<code className="bg-gray-700 px-1 rounded">custom.pt</code>) fica disponível no seletor acima.
            </p>
          </div>

          <div className="p-4 space-y-3">
            <div className="flex items-start justify-between gap-4">
              <div className="space-y-0.5">
                <p className="text-sm text-gray-300">
                  {annCount === null ? '…' : annCount} bounding box{annCount !== 1 ? 'es' : ''}
                  {' · '}
                  {labelCount === null ? '…' : labelCount} evento{labelCount !== 1 ? 's' : ''} rotulado{labelCount !== 1 ? 's' : ''}
                </p>
                <p className="text-xs text-gray-500">
                  Bounding boxes + labels de texto são usados no treino
                </p>
              </div>
              <button
                type="button"
                disabled={(!annCount && !labelCount) || (ftStatus?.status === 'running' || ftStatus?.status === 'pending')}
                onClick={handleStartFinetune}
                className="px-4 py-2 bg-violet-600 hover:bg-violet-500 text-white text-sm rounded transition-colors disabled:opacity-40 disabled:cursor-not-allowed shrink-0"
              >
                Treinar agora
              </button>
            </div>
            <div className="flex items-center gap-3">
              <label className="text-xs text-gray-400 shrink-0">Épocas</label>
              <input
                type="number"
                min={1}
                max={200}
                value={epochs}
                onChange={e => setEpochs(Math.min(200, Math.max(1, Number(e.target.value) || 20)))}
                className="w-20 bg-gray-700 text-gray-200 text-sm rounded px-2 py-1 border border-gray-600 focus:outline-none focus:border-blue-500"
              />
              <p className="text-xs text-gray-500">
                Mais épocas = aprende melhor, mas demora mais e pode decorar exemplos (overfitting) com poucos dados. Para &lt; 200 exemplos, 20–50 épocas costuma ser o ideal.
              </p>
            </div>
          </div>

          {ftStatus && (
            <div className="p-4 space-y-2">
              {(ftStatus.status === 'running' || ftStatus.status === 'pending') && (
                <>
                  <div className="flex items-center justify-between text-xs text-gray-400">
                    <span>{ftStatus.status === 'pending' ? 'Iniciando…' : `Época ${ftStatus.epoch} / ${ftStatus.total_epochs}`}</span>
                    <div className="flex items-center gap-3">
                      <span>{ftStatus.status === 'running' ? `${Math.round((ftStatus.epoch / ftStatus.total_epochs) * 100)}%` : ''}</span>
                      <button
                        type="button"
                        onClick={handleCancelFinetune}
                        className="px-2 py-0.5 text-xs bg-gray-700 hover:bg-red-900/60 text-gray-400 hover:text-red-300 border border-gray-600 hover:border-red-700/50 rounded transition-colors"
                      >
                        Cancelar
                      </button>
                    </div>
                  </div>
                  <div className="w-full bg-gray-700 rounded-full h-2">
                    <div
                      className="bg-violet-500 h-2 rounded-full transition-all"
                      style={{ width: `${Math.round((ftStatus.epoch / ftStatus.total_epochs) * 100)}%` }}
                    />
                  </div>
                </>
              )}
              {ftStatus.status === 'done' && (
                <p className="text-sm text-green-400">Treino concluído. Modelo salvo como <code className="bg-gray-700 px-1 rounded">custom</code>.</p>
              )}
              {ftStatus.status === 'error' && (
                <p className="text-sm text-red-400">{ftError || ftStatus.error || 'Erro no treino'}</p>
              )}
            </div>
          )}

          {ftError && !ftStatus && (
            <div className="p-4">
              <p className="text-sm text-red-400">{ftError}</p>
            </div>
          )}
        </div>
        <div className="bg-gray-800 rounded-lg border border-gray-700 divide-y divide-gray-700">
          <div className="p-4 flex items-center justify-between gap-4 flex-wrap">
            <div>
              <h4 className="text-sm font-semibold text-gray-200 mb-1">Rotular eventos</h4>
              <p className="text-xs text-gray-400">Atribua labels de texto aos eventos de movimento para curadoria do dataset de treino.</p>
            </div>
            <div className="flex items-center gap-3 flex-wrap">
              <select
                className="bg-gray-700 text-gray-200 text-sm rounded px-3 py-1.5 border border-gray-600 focus:outline-none focus:border-blue-500"
                value={labelCamID}
                onChange={e => { setLabelCamID(e.target.value); setLabelPage(1); setLabelEvents(null); clearSelection() }}
              >
                <option value="">Selecionar câmera…</option>
                {cameras.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}
              </select>
              <input
                type="text"
                placeholder="Buscar label…"
                value={labelSearch}
                onChange={e => { setLabelSearch(e.target.value); setLabelPage(1); setLabelEvents(null); clearSelection() }}
                className="bg-gray-700 text-gray-200 text-sm rounded px-3 py-1.5 border border-gray-600 focus:outline-none focus:border-blue-500 w-40"
              />
              <label className={`flex items-center gap-1.5 text-xs cursor-pointer select-none ${labelSearch ? 'text-gray-600 cursor-not-allowed' : 'text-gray-400'}`}>
                <input
                  type="checkbox"
                  className="accent-blue-500"
                  checked={unlabeledOnly && !labelSearch}
                  disabled={!!labelSearch}
                  onChange={e => { setUnlabeledOnly(e.target.checked); setLabelPage(1); setLabelEvents(null); clearSelection() }}
                />
                Sem label
              </label>
            </div>
          </div>

          {labelCamID && (
            <div>
              {labelLoading && (
                <div className="p-6 text-center text-xs text-gray-500">Carregando…</div>
              )}
              {!labelLoading && labelEvents?.length === 0 && (
                <div className="p-6 text-center text-xs text-gray-500">
                  {unlabeledOnly ? 'Nenhum evento sem label.' : 'Nenhum evento encontrado.'}
                </div>
              )}
              {!labelLoading && (labelEvents?.length ?? 0) > 0 && (
                <>
                  <div className="flex items-center gap-3 px-4 py-2 bg-gray-900/40 border-b border-gray-700 text-xs text-gray-400">
                    <label className="flex items-center gap-2 cursor-pointer select-none">
                      <input
                        type="checkbox"
                        className="accent-blue-500"
                        checked={(labelEvents?.length ?? 0) > 0 && labelEvents!.every(e => selected.has(e.id))}
                        onChange={e => e.target.checked ? selectAllOnPage() : clearSelection()}
                      />
                      Selecionar todos da página
                    </label>
                    {selected.size > 0 && (
                      <span className="text-blue-400">{selected.size} selecionado{selected.size !== 1 ? 's' : ''}</span>
                    )}
                  </div>
                  {selected.size > 0 && (
                    <div className="flex flex-wrap items-center gap-2 px-4 py-2 bg-blue-900/20 border-b border-blue-700/40 sticky top-0 z-10">
                      <input
                        type="text"
                        placeholder="label para aplicar em lote…"
                        value={bulkLabel}
                        onChange={e => setBulkLabel(e.target.value)}
                        className="flex-1 min-w-[10rem] bg-gray-800 text-gray-200 text-sm rounded px-2 py-1 border border-gray-600 focus:outline-none focus:border-blue-500"
                      />
                      <button
                        type="button"
                        disabled={bulkBusy}
                        onClick={() => setBulkConfirm({ action: 'label', label: bulkLabel })}
                        className="px-3 py-1 text-xs bg-blue-600 hover:bg-blue-500 text-white rounded disabled:opacity-40"
                      >
                        Aplicar label
                      </button>
                      <button
                        type="button"
                        disabled={bulkBusy}
                        onClick={() => setBulkConfirm({ action: 'delete' })}
                        className="px-3 py-1 text-xs bg-red-600 hover:bg-red-500 text-white rounded disabled:opacity-40"
                      >
                        Excluir
                      </button>
                      <button
                        type="button"
                        disabled={bulkBusy}
                        onClick={clearSelection}
                        className="px-3 py-1 text-xs text-gray-400 hover:text-white border border-gray-600 rounded disabled:opacity-40"
                      >
                        Limpar
                      </button>
                      {bulkError && <span className="text-xs text-red-400">{bulkError}</span>}
                    </div>
                  )}
                  <ul className="divide-y divide-gray-700">
                    {labelEvents!.map(ev => {
                      const state = labelSaveState[ev.id]
                      const borderCls = state === 'saved'
                        ? 'border-green-500'
                        : state === 'error'
                        ? 'border-red-500'
                        : 'border-gray-600'
                      const isSelected = selected.has(ev.id)
                      return (
                        <li key={ev.id} className={`flex items-center gap-3 px-4 py-2 ${isSelected ? 'bg-blue-900/10' : ''}`}>
                          <input
                            type="checkbox"
                            className="accent-blue-500 flex-shrink-0"
                            checked={isSelected}
                            onChange={() => toggleSelect(ev.id)}
                          />
                          {ev.frame ? (
                            <button
                              type="button"
                              onClick={() => openZoomModal(frameURL(labelCamID, ev.time, ev.frame!), ev.id, labelInputs[ev.id] ?? ev.label ?? '')}
                              className="flex-shrink-0 rounded overflow-hidden focus:outline-none focus:ring-2 focus:ring-blue-500 hover:opacity-80 transition-opacity"
                            >
                              <img
                                src={frameURL(labelCamID, ev.time, ev.frame)}
                                className="w-40 h-24 object-cover bg-gray-900"
                                alt=""
                              />
                            </button>
                          ) : (
                            <div className="w-40 h-24 rounded bg-gray-900 flex-shrink-0 flex items-center justify-center text-gray-600 text-xs">
                              sem frame
                            </div>
                          )}
                          <div className="flex-1 min-w-0">
                            <p className="text-xs text-gray-500 mb-1">
                              {new Date(ev.time).toLocaleString()}
                              <span className="ml-2 text-gray-600">score: {ev.score.toFixed(2)}</span>
                            </p>
                            <input
                              type="text"
                              placeholder="label…"
                              value={labelInputs[ev.id] ?? ''}
                              onChange={e => setLabelInputs(s => ({ ...s, [ev.id]: e.target.value }))}
                              onBlur={() => handleLabelBlur(ev.id)}
                              className={`w-full bg-gray-700 text-gray-200 text-sm rounded px-2 py-1 border ${borderCls} focus:outline-none focus:border-blue-500 transition-colors`}
                            />
                          </div>
                          <button
                            type="button"
                            onClick={() => setRowDeleteConfirm(ev)}
                            title="Excluir este evento"
                            className="flex-shrink-0 w-7 h-7 flex items-center justify-center text-gray-500 hover:text-red-400 hover:bg-red-500/10 rounded transition-colors"
                          >
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="w-4 h-4">
                              <path strokeLinecap="round" strokeLinejoin="round" d="M6 6l12 12M6 18L18 6" />
                            </svg>
                          </button>
                        </li>
                      )
                    })}
                  </ul>
                </>
              )}

              {labelTotal > LIMIT && (
                <div className="p-3 flex items-center justify-between border-t border-gray-700">
                  <span className="text-xs text-gray-500">{labelTotal} eventos · página {labelPage} de {Math.ceil(labelTotal / LIMIT)}</span>
                  <div className="flex gap-2">
                    <button
                      onClick={() => { setLabelPage(p => Math.max(1, p - 1)); setLabelEvents(null); clearSelection() }}
                      disabled={labelPage === 1}
                      className="px-3 py-1 text-xs bg-gray-700 hover:bg-gray-600 text-gray-300 rounded disabled:opacity-40 disabled:cursor-not-allowed"
                    >
                      ← anterior
                    </button>
                    <button
                      onClick={() => { setLabelPage(p => p + 1); setLabelEvents(null); clearSelection() }}
                      disabled={labelPage >= Math.ceil(labelTotal / LIMIT)}
                      className="px-3 py-1 text-xs bg-gray-700 hover:bg-gray-600 text-gray-300 rounded disabled:opacity-40 disabled:cursor-not-allowed"
                    >
                      próxima →
                    </button>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {zoomEvent && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/80"
          onClick={() => closeZoomModal()}
        >
          <div className="flex flex-col gap-3 items-center" onClick={e => e.stopPropagation()}>
            <div className="relative rounded overflow-hidden shadow-2xl" style={{ maxWidth: '90vw', maxHeight: '75vh' }}>
              <img
                src={zoomEvent.src}
                className="block max-w-full max-h-full"
                alt=""
                draggable={false}
              />
              <BboxCanvas
                box={annBox ?? (annSaveOk ? null : existingAnn)}
                onChange={handleAnnBoxChange}
                readonly={annSaveOk}
                className="absolute inset-0 w-full h-full select-none"
              />
            </div>

            <div className="flex items-center gap-2 w-full max-w-md">
              {annSaveOk && (
                <span className="text-xs text-emerald-400">Anotação salva</span>
              )}
              {!annSaveOk && annBox && annBox.w > 0.01 && annBox.h > 0.01 && (
                <>
                  <input
                    type="text"
                    placeholder="label da região…"
                    value={annLabel}
                    onChange={e => setAnnLabel(e.target.value)}
                    onKeyDown={e => e.key === 'Enter' && saveAnnotation()}
                    autoFocus
                    className="flex-1 bg-gray-800 text-gray-200 text-sm rounded px-3 py-1.5 border border-gray-600 focus:outline-none focus:border-emerald-500"
                  />
                  <button
                    onClick={saveAnnotation}
                    disabled={annSaving}
                    className="px-3 py-1.5 text-sm bg-emerald-700 hover:bg-emerald-600 text-white rounded disabled:opacity-60 disabled:cursor-not-allowed"
                  >
                    {annSaving ? 'Salvando...' : 'Salvar'}
                  </button>
                  <button
                    onClick={() => setAnnBox(null)}
                    className="px-3 py-1.5 text-sm bg-gray-700 hover:bg-gray-600 text-gray-300 rounded"
                  >
                    Cancelar
                  </button>
                </>
              )}
              {!annSaveOk && !annBox && existingAnn && (
                <span className="text-xs text-gray-400 flex items-center gap-2">
                  {existingAnnLabel
                    ? <><span className="font-medium text-gray-300">{existingAnnLabel}</span> · Arraste para substituir</>
                    : 'Região salva · Arraste para substituir'
                  }
                  {existingAnnId && (
                    <button
                      onClick={deleteAnnotation}
                      className="px-2 py-0.5 text-xs text-red-400 hover:text-red-300 hover:bg-red-900/30 border border-red-700/40 rounded transition-colors"
                    >
                      Excluir anotação
                    </button>
                  )}
                </span>
              )}
              {!annSaveOk && !annBox && !existingAnn && (
                <span className="text-xs text-gray-500">Arraste para marcar · mova · redimensione · rotacione</span>
              )}
              <button
                onClick={() => closeZoomModal()}
                className="ml-auto px-3 py-1.5 text-sm bg-gray-700 hover:bg-gray-600 text-gray-300 rounded"
              >
                Fechar
              </button>
            </div>
          </div>
        </div>
      )}

      <ConfirmDialog
        open={bulkConfirm?.action === 'delete'}
        title="Excluir eventos"
        message={`Excluir ${selected.size} evento${selected.size !== 1 ? 's' : ''}? Esta ação não pode ser desfeita.`}
        confirmLabel={bulkBusy ? 'Excluindo…' : 'Excluir'}
        danger
        onConfirm={executeBulkDelete}
        onCancel={() => { if (!bulkBusy) setBulkConfirm(null) }}
      />
      <ConfirmDialog
        open={!!rowDeleteConfirm}
        title="Excluir evento"
        message={
          rowDeleteConfirm
            ? `Excluir o evento de ${new Date(rowDeleteConfirm.time).toLocaleString()}? Esta ação não pode ser desfeita.`
            : ''
        }
        confirmLabel={rowDeleteBusy ? 'Excluindo…' : 'Excluir'}
        danger
        onConfirm={executeRowDelete}
        onCancel={() => { if (!rowDeleteBusy) setRowDeleteConfirm(null) }}
      />
      <ConfirmDialog
        open={bulkConfirm?.action === 'label'}
        title={bulkLabel ? 'Aplicar label' : 'Remover label'}
        message={
          bulkLabel
            ? `Aplicar label "${bulkLabel}" em ${selected.size} evento${selected.size !== 1 ? 's' : ''}?`
            : `Remover label de ${selected.size} evento${selected.size !== 1 ? 's' : ''}?`
        }
        confirmLabel={bulkBusy ? 'Aplicando…' : 'Aplicar'}
        onConfirm={executeBulkLabel}
        onCancel={() => { if (!bulkBusy) setBulkConfirm(null) }}
      />
    </SettingsLayout>
  )
}
