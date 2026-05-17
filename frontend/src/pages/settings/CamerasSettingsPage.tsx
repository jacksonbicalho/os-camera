import { useState, useEffect, useCallback } from 'react'
import { Link, useNavigate, useLocation } from 'react-router-dom'
import SettingsLayout from '../../components/SettingsLayout'
import ConfirmDialog from '../../components/ConfirmDialog'
import { authHeaders, clearToken, getRole } from '../../auth'

interface MotionConfig {
  enabled: boolean
  threshold: number
  fps: number
  cooldown_seconds: number
  capture_width?: number
  capture_height?: number
  playback_lead_seconds?: number
}

interface Camera {
  id: string
  rtsp_url: string
  chunk_duration: string
  reconnect_interval: string
  video_codec: string
  has_audio: boolean | null
  width: number
  height: number
  display_order: number
  hls_video_mode: string
  motion: MotionConfig | null
}

interface CameraFormData {
  id: string
  rtsp_url: string
  chunk_duration: string
  reconnect_interval: string
  video_codec: string
  has_audio: '' | 'true' | 'false'
  resolution: string
  display_order: string
  hls_video_mode: string
  motion_enabled: boolean
  motion_threshold: string
  motion_fps: string
  motion_cooldown: string
  motion_capture_auto: boolean
  motion_capture_pct: number
  motion_playback_lead: string
}

const RESOLUTIONS = [
  { label: 'Auto', value: '0x0' },
  { label: '352 × 288 (CIF)', value: '352x288' },
  { label: '640 × 480 (VGA)', value: '640x480' },
  { label: '720 × 576 (D1)', value: '720x576' },
  { label: '1280 × 720 (HD)', value: '1280x720' },
  { label: '1920 × 1080 (Full HD)', value: '1920x1080' },
  { label: '2560 × 1440 (2K)', value: '2560x1440' },
  { label: '3840 × 2160 (4K)', value: '3840x2160' },
]

function encodeResolution(w: number, h: number): string {
  if (w === 0 || h === 0) return '0x0'
  const match = RESOLUTIONS.find(r => r.value === `${w}x${h}`)
  return match ? match.value : `${w}x${h}`
}

function decodeResolution(value: string): { width: number; height: number } {
  const [w, h] = value.split('x').map(Number)
  return { width: w || 0, height: h || 0 }
}

function capturePct(capW: number, streamW: number): number {
  if (capW > 0 && streamW > 0) return Math.round(capW / streamW * 100)
  return 25
}

function emptyForm(cam?: Camera): CameraFormData {
  if (!cam) {
    return {
      id: '', rtsp_url: '', chunk_duration: '5m', reconnect_interval: '30s',
      video_codec: '', has_audio: '', resolution: '0x0', display_order: '0',
      hls_video_mode: 'auto',
      motion_enabled: false, motion_threshold: '0.02', motion_fps: '2', motion_cooldown: '30',
      motion_capture_auto: true, motion_capture_pct: 25, motion_playback_lead: '10',
    }
  }
  const capW = cam.motion?.capture_width ?? 0
  const auto = capW === 0
  return {
    id: cam.id,
    rtsp_url: cam.rtsp_url,
    chunk_duration: cam.chunk_duration,
    reconnect_interval: cam.reconnect_interval,
    video_codec: cam.video_codec ?? '',
    has_audio: cam.has_audio == null ? '' : cam.has_audio ? 'true' : 'false',
    resolution: encodeResolution(cam.width ?? 0, cam.height ?? 0),
    display_order: String(cam.display_order ?? 0),
    hls_video_mode: cam.hls_video_mode || 'auto',
    motion_enabled: cam.motion?.enabled ?? false,
    motion_threshold: String(cam.motion?.threshold ?? 0.02),
    motion_fps: String(cam.motion?.fps ?? 2),
    motion_cooldown: String(cam.motion?.cooldown_seconds ?? 30),
    motion_capture_auto: auto,
    motion_capture_pct: capturePct(capW, cam.width ?? 0),
    motion_playback_lead: String(cam.motion?.playback_lead_seconds ?? 10),
  }
}

function formToPayload(f: CameraFormData, includeID = true) {
  const { width, height } = decodeResolution(f.resolution)
  const payload: Record<string, unknown> = {
    rtsp_url: f.rtsp_url,
    chunk_duration: f.chunk_duration || '5m',
    reconnect_interval: f.reconnect_interval || '30s',
    video_codec: f.video_codec,
    has_audio: f.has_audio === '' ? null : f.has_audio === 'true',
    width,
    height,
    display_order: parseInt(f.display_order) || 0,
    hls_video_mode: f.hls_video_mode || 'auto',
    motion: {
      enabled: f.motion_enabled,
      threshold: parseFloat(f.motion_threshold) || 0.02,
      fps: parseInt(f.motion_fps) || 2,
      cooldown_seconds: parseInt(f.motion_cooldown) || 30,
      capture_width: f.motion_capture_auto ? 0 : Math.round(width * f.motion_capture_pct / 100),
      capture_height: f.motion_capture_auto ? 0 : Math.round(height * f.motion_capture_pct / 100),
      playback_lead_seconds: parseInt(f.motion_playback_lead) || 10,
    },
  }
  if (includeID) payload.id = f.id
  return payload
}

interface CameraFormProps {
  initial?: Camera
  onSave: (data: CameraFormData) => Promise<void>
  onCancel: () => void
  saving: boolean
}

function CameraForm({ initial, onSave, onCancel, saving }: CameraFormProps) {
  const [form, setForm] = useState<CameraFormData>(() => emptyForm(initial))
  const isEdit = !!initial

  const set = (field: keyof CameraFormData, value: string | boolean | number) =>
    setForm(prev => ({ ...prev, [field]: value }))

  const { width: streamW, height: streamH } = decodeResolution(form.resolution)
  const previewW = form.motion_capture_auto
    ? (streamW > 0 ? Math.round(streamW / 4) : null)
    : (streamW > 0 ? Math.round(streamW * form.motion_capture_pct / 100) : null)
  const previewH = form.motion_capture_auto
    ? (streamH > 0 ? Math.round(streamH / 4) : null)
    : (streamH > 0 ? Math.round(streamH * form.motion_capture_pct / 100) : null)

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSave(form)
  }

  const inputClass = "w-full bg-gray-950 border border-gray-700 rounded px-3 py-1.5 text-sm text-gray-200 focus:outline-none focus:border-blue-500"
  const labelClass = "block text-xs text-gray-400 mb-1"

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4">
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <div>
          <label className={labelClass}>ID</label>
          <input
            value={form.id}
            onChange={e => set('id', e.target.value)}
            disabled={isEdit}
            required
            className={`${inputClass} ${isEdit ? 'opacity-50 cursor-not-allowed' : ''}`}
          />
        </div>
        <div className="sm:col-span-1">
          <label className={labelClass}>RTSP URL</label>
          <input
            value={form.rtsp_url}
            onChange={e => set('rtsp_url', e.target.value)}
            required
            className={inputClass}
            placeholder="rtsp://usuario:senha@ip:554/stream"
          />
        </div>
        <div>
          <label className={labelClass}>Duração do chunk</label>
          <input value={form.chunk_duration} onChange={e => set('chunk_duration', e.target.value)} className={inputClass} placeholder="5m" />
        </div>
        <div>
          <label className={labelClass}>Intervalo de reconexão</label>
          <input value={form.reconnect_interval} onChange={e => set('reconnect_interval', e.target.value)} className={inputClass} placeholder="30s" />
        </div>
        <div>
          <label className={labelClass}>Codec de vídeo</label>
          <select value={form.video_codec} onChange={e => set('video_codec', e.target.value)} className={inputClass}>
            <option value="">Auto (ffprobe detecta)</option>
            <option value="h264">H.264 / AVC</option>
            <option value="hevc">HEVC / H.265</option>
            <option value="mjpeg">MJPEG</option>
            <option value="mpeg4">MPEG-4</option>
          </select>
        </div>
        <div>
          <label className={labelClass}>Áudio</label>
          <select value={form.has_audio} onChange={e => set('has_audio', e.target.value)} className={inputClass}>
            <option value="">Auto</option>
            <option value="true">Sim</option>
            <option value="false">Não</option>
          </select>
        </div>
        <div>
          <label className={labelClass}>Resolução</label>
          <select value={form.resolution} onChange={e => set('resolution', e.target.value)} className={inputClass}>
            {RESOLUTIONS.map(r => (
              <option key={r.value} value={r.value}>{r.label}</option>
            ))}
            {!RESOLUTIONS.find(r => r.value === form.resolution) && (
              <option value={form.resolution}>{form.resolution.replace('x', ' × ')}</option>
            )}
          </select>
        </div>
        <div>
          <label className={labelClass}>Ordem de exibição</label>
          <input type="number" value={form.display_order} onChange={e => set('display_order', e.target.value)} className={inputClass} />
        </div>
        <div>
          <label className={labelClass}>Modo de vídeo HLS</label>
          <select value={form.hls_video_mode} onChange={e => set('hls_video_mode', e.target.value)} className={inputClass}>
            <option value="auto">Auto (detecta via ffprobe)</option>
            <option value="h264">H.264 (sempre transcodifica)</option>
            <option value="copy">Cópia (sem transcodificação)</option>
          </select>
        </div>
      </div>

      <div className="border-t border-gray-800 pt-3">
        <p className="text-xs font-medium text-gray-400 mb-3">Detecção de movimento</p>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div className="flex items-center gap-2 sm:col-span-2">
            <input
              type="checkbox"
              id="motion_enabled"
              checked={form.motion_enabled}
              onChange={e => set('motion_enabled', e.target.checked)}
              className="accent-blue-500"
            />
            <label htmlFor="motion_enabled" className="text-xs text-gray-400 cursor-pointer">Habilitado</label>
          </div>
          {form.motion_enabled && (
            <>
              <div>
                <label className={labelClass}>Limiar (0.0 – 1.0)</label>
                <input type="number" step="0.001" min="0" max="1" value={form.motion_threshold} onChange={e => set('motion_threshold', e.target.value)} className={inputClass} />
              </div>
              <div>
                <label className={labelClass}>FPS de análise</label>
                <input type="number" min="1" value={form.motion_fps} onChange={e => set('motion_fps', e.target.value)} className={inputClass} />
              </div>
              <div>
                <label className={labelClass}>Cooldown (segundos)</label>
                <input type="number" min="0" value={form.motion_cooldown} onChange={e => set('motion_cooldown', e.target.value)} className={inputClass} />
              </div>
              <div>
                <label className={labelClass}>Segundos antes do evento</label>
                <input type="number" min="1" max="300" value={form.motion_playback_lead} onChange={e => set('motion_playback_lead', e.target.value)} className={inputClass} />
              </div>
              <div className="sm:col-span-2">
                <label className={labelClass}>Resolução de análise</label>
                <div className="flex items-center gap-2 mb-2">
                  <input
                    type="checkbox"
                    id="motion_capture_auto"
                    checked={form.motion_capture_auto}
                    onChange={e => set('motion_capture_auto', e.target.checked)}
                    className="accent-blue-500"
                  />
                  <label htmlFor="motion_capture_auto" className="text-xs text-gray-400 cursor-pointer">
                    Automático (stream ÷ 4{previewW !== null ? ` → ${previewW} × ${previewH} px` : ''})
                  </label>
                </div>
                {!form.motion_capture_auto && (
                  <div className="flex flex-col gap-1.5">
                    <div className="flex items-center gap-3">
                      <input
                        type="range"
                        min={5} max={100} step={5}
                        value={form.motion_capture_pct}
                        onChange={e => set('motion_capture_pct', parseInt(e.target.value))}
                        className="flex-1 accent-blue-500"
                      />
                      <span className="text-xs text-gray-300 font-mono w-10 text-right">{form.motion_capture_pct}%</span>
                    </div>
                    {previewW !== null
                      ? <p className="text-xs text-gray-500">→ {previewW} × {previewH} px</p>
                      : <p className="text-xs text-gray-600">Configure largura e altura do stream para ver a resolução em pixels</p>
                    }
                  </div>
                )}
              </div>
            </>
          )}
        </div>
      </div>

      <div className="flex gap-2">
        <button
          type="submit"
          disabled={saving}
          className="px-4 py-1.5 text-xs bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded transition-colors"
        >
          {saving ? 'Salvando...' : 'Salvar'}
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="px-4 py-1.5 text-xs text-gray-300 hover:text-white border border-gray-600 rounded transition-colors"
        >
          Cancelar
        </button>
      </div>
    </form>
  )
}

export default function CamerasSettingsPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const isAdmin = getRole() === 'admin'
  const isNewRoute = location.pathname === '/settings/cameras/new'
  const [cameras, setCameras] = useState<Camera[]>([])
  const [loading, setLoading] = useState(isAdmin)
  const [creating, setCreating] = useState(isNewRoute)
  const locationState = location.state as { editId?: string; from?: string } | null
  const [editingId, setEditingId] = useState<string | null>(locationState?.editId ?? null)
  const from = locationState?.from ?? null
  const [saving, setSaving] = useState(false)
  const [deleteId, setDeleteId] = useState<string | null>(null)
  const [deleteData, setDeleteData] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [noDb, setNoDb] = useState(false)

  const reloadCameras = useCallback(async () => {
    const res = await fetch('/api/settings/cameras', { headers: authHeaders() })
    if (res.status === 401) { clearToken(); navigate('/login', { replace: true }); return }
    if (res.status === 503) { setNoDb(true); return }
    if (res.ok) setCameras(await res.json())
  }, [navigate])

  useEffect(() => {
    if (!isAdmin) return
    fetch('/api/settings/cameras', { headers: authHeaders() })
      .then(async res => {
        if (res.status === 401) { clearToken(); navigate('/login', { replace: true }); return }
        if (res.status === 503) { setNoDb(true); return }
        if (res.ok) setCameras(await res.json())
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [isAdmin, navigate])

  const handleCreate = async (data: CameraFormData) => {
    setSaving(true); setError(null)
    try {
      const res = await fetch('/api/settings/cameras', {
        method: 'POST',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify(formToPayload(data, true)),
      })
      if (!res.ok) { setError((await res.text()).trim() || 'Erro ao criar câmera'); return }
      if (isNewRoute) {
        navigate('/settings/cameras', { replace: true })
        return
      }
      await reloadCameras()
      setCreating(false)
    } finally { setSaving(false) }
  }

  const handleUpdate = async (id: string, data: CameraFormData) => {
    setSaving(true); setError(null)
    try {
      const res = await fetch(`/api/settings/cameras/${id}`, {
        method: 'PUT',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify(formToPayload(data, false)),
      })
      if (!res.ok) { setError((await res.text()).trim() || 'Erro ao atualizar câmera'); return }
      await reloadCameras()
      setEditingId(null)
    } finally { setSaving(false) }
  }

  const handleDelete = async () => {
    if (!deleteId) return
    const url = deleteData
      ? `/api/settings/cameras/${deleteId}?delete_data=true`
      : `/api/settings/cameras/${deleteId}`
    try {
      await fetch(url, { method: 'DELETE', headers: authHeaders() })
      await reloadCameras()
    } finally { setDeleteId(null); setDeleteData(false) }
  }

  const camToDelete = cameras.find(c => c.id === deleteId)

  // Viewer: show read-only list (the old behavior)
  if (!isAdmin) {
    return (
      <SettingsLayout>
        <h2 className="text-lg font-semibold text-gray-200 mb-6">Câmeras</h2>
        {loading ? (
          <p className="text-gray-500 text-sm">Carregando...</p>
        ) : cameras.length === 0 ? (
          <p className="text-gray-500 text-sm">Nenhuma câmera configurada.</p>
        ) : (
          <div className="flex flex-col gap-2">
            {cameras.map(cam => (
              <Link
                key={cam.id}
                to={`/settings/cameras/${cam.id}`}
                className="flex items-center justify-between bg-gray-900 border border-gray-800 rounded-lg px-5 py-4 hover:border-blue-600 transition-colors"
              >
                <span className="text-sm font-mono text-gray-200">{cam.id}</span>
                <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                </svg>
              </Link>
            ))}
          </div>
        )}
      </SettingsLayout>
    )
  }

  return (
    <SettingsLayout>
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          {from && (
            <button
              onClick={() => navigate(-1)}
              className="text-sm text-blue-400 hover:text-blue-300 transition-colors"
            >
              ← Voltar
            </button>
          )}
          <h2 className="text-lg font-semibold text-gray-200">Câmeras</h2>
        </div>
        {!creating && !noDb && (
          <button
            onClick={() => { setCreating(true); setEditingId(null); setError(null) }}
            className="px-3 py-1.5 text-xs bg-blue-600 hover:bg-blue-500 text-white rounded transition-colors"
          >
            + Nova câmera
          </button>
        )}
      </div>

      {noDb && (
        <p className="text-gray-500 text-sm">Gerenciamento de câmeras requer banco de dados configurado.</p>
      )}

      {error && (
        <div className="mb-4 px-3 py-2 bg-red-900/30 border border-red-700/50 rounded text-xs text-red-400">
          {error}
        </div>
      )}

      {creating && (
        <div className="mb-4 bg-gray-900 border border-gray-700 rounded-lg p-4">
          <p className="text-xs font-medium text-gray-400 mb-3">Nova câmera</p>
          <CameraForm
            onSave={handleCreate}
            onCancel={() => {
              if (isNewRoute) { navigate('/settings/cameras', { replace: true }); return }
              setCreating(false); setError(null)
            }}
            saving={saving}
          />
        </div>
      )}

      {loading ? (
        <p className="text-gray-500 text-sm">Carregando...</p>
      ) : cameras.length === 0 && !noDb ? (
        <p className="text-gray-500 text-sm">Nenhuma câmera configurada.</p>
      ) : (
        <div className="flex flex-col gap-2">
          {cameras.map(cam => (
            <div key={cam.id} className="bg-gray-900 border border-gray-800 rounded-lg px-4 py-3">
              {editingId === cam.id ? (
                <CameraForm
                  initial={cam}
                  onSave={data => handleUpdate(cam.id, data)}
                  onCancel={() => { setEditingId(null); setError(null) }}
                  saving={saving}
                />
              ) : (
                <div className="flex items-center justify-between gap-3">
                  <div className="flex items-center gap-3 min-w-0">
                    <Link
                      to={`/settings/cameras/${cam.id}`}
                      className="text-sm font-mono text-gray-200 hover:text-blue-400 transition-colors truncate"
                    >
                      {cam.id}
                    </Link>
                    <span className="text-xs text-gray-600 truncate hidden sm:block">
                      {cam.rtsp_url.replace(/:[^:@]+@/, ':***@')}
                    </span>
                    {cam.motion?.enabled && (
                      <span className="px-2 py-0.5 text-xs rounded-full bg-green-900/40 text-green-400 border border-green-700/40 shrink-0">
                        motion
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-1 shrink-0">
                    <button
                      onClick={() => { setEditingId(cam.id); setCreating(false); setError(null) }}
                      className="px-3 py-1 text-xs text-gray-400 hover:text-white border border-gray-700 rounded transition-colors"
                    >
                      Editar
                    </button>
                    <button
                      onClick={() => setDeleteId(cam.id)}
                      className="px-3 py-1 text-xs text-red-500 hover:text-red-400 border border-gray-700 rounded transition-colors"
                    >
                      Remover
                    </button>
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      <ConfirmDialog
        open={deleteId != null}
        title="Remover câmera"
        message={`Remover câmera "${camToDelete?.id}"?`}
        confirmLabel="Remover"
        danger
        onConfirm={handleDelete}
        onCancel={() => { setDeleteId(null); setDeleteData(false) }}
      >
        <label className="flex items-center gap-2 cursor-pointer">
          <input
            type="checkbox"
            checked={deleteData}
            onChange={e => setDeleteData(e.target.checked)}
            className="accent-red-500"
          />
          <span className="text-xs text-gray-300">Apagar também as gravações do disco</span>
        </label>
      </ConfirmDialog>
    </SettingsLayout>
  )
}
