import { useState, useEffect, useCallback, useRef } from 'react'
import { Link, useNavigate, useLocation, useSearchParams } from 'react-router-dom'
import SettingsLayout from '../../components/SettingsLayout'
import ConfirmDialog from '../../components/ConfirmDialog'
import CameraForm from '../../components/CameraForm'
import { type Camera, type CameraFormData, formToPayload } from '../../components/cameraFormUtils'
import { authHeaders, onUnauthorized, getRole, getToken } from '../../auth'
import { Plus, GripVertical, ChevronRight, Pencil, Trash2 } from '../../components/Icons'

export default function CamerasSettingsPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const [searchParams] = useSearchParams()
  const isAdmin = getRole() === 'admin'
  const isNewRoute = location.pathname === '/settings/cameras/new'
  const prefillRTSP = searchParams.get('prefill_rtsp') ?? ''
  const prefillName = searchParams.get('prefill_name') ?? ''
  const [cameras, setCameras] = useState<Camera[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(isNewRoute)
  const [saving, setSaving] = useState(false)
  const [deleteId, setDeleteId] = useState<string | null>(null)
  const [deleteData, setDeleteData] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [noDb, setNoDb] = useState(false)
  const [dragOverId, setDragOverId] = useState<string | null>(null)
  const dragIdRef = useRef<string | null>(null)

  const reloadCameras = useCallback(async () => {
    const res = await fetch('/api/settings/cameras', { headers: authHeaders() })
    if (res.status === 401) { onUnauthorized(); return }
    if (res.status === 503) { setNoDb(true); return }
    if (res.ok) setCameras(await res.json())
  }, [])

  useEffect(() => {
    if (!isAdmin) return
    fetch('/api/settings/cameras', { headers: authHeaders() })
      .then(async res => {
        if (res.status === 401) { onUnauthorized(); return }
        if (res.status === 503) { setNoDb(true); return }
        if (res.ok) setCameras(await res.json())
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [isAdmin, navigate])

  useEffect(() => {
    if (isAdmin) return
    fetch('/api/cameras', { headers: authHeaders() })
      .then(async res => {
        if (res.status === 401) { onUnauthorized(); return }
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
        body: JSON.stringify(formToPayload(data)),
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

  const handleDrop = async (targetId: string) => {
    const sourceId = dragIdRef.current
    dragIdRef.current = null
    setDragOverId(null)
    if (!sourceId || sourceId === targetId) return

    const reordered = [...cameras]
    const fromIdx = reordered.findIndex(c => c.id === sourceId)
    const toIdx = reordered.findIndex(c => c.id === targetId)
    if (fromIdx < 0 || toIdx < 0) return

    reordered.splice(toIdx, 0, reordered.splice(fromIdx, 1)[0])
    setCameras(reordered)

    await fetch('/api/settings/cameras/reorder', {
      method: 'PUT',
      headers: { ...authHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify({ ids: reordered.map(c => c.id) }),
    })
  }

  const camToDelete = cameras.find(c => c.id === deleteId)

  if (!isAdmin) {
    return (
      <SettingsLayout>
        <div className="flex items-center justify-between mb-6">
          <h3 className="text-h2 font-semibold text-foreground">Câmeras</h3>
        </div>
        {loading ? (
          <p className="text-muted-foreground text-sm">Carregando...</p>
        ) : cameras.length === 0 ? (
          <p className="text-muted-foreground text-sm">Nenhuma câmera disponível.</p>
        ) : (
          <div className="flex flex-col gap-2">
            {cameras.map(cam => (
              <Link
                key={cam.id}
                to={`/settings/cameras/${cam.id}`}
                className="flex items-center gap-4 bg-surface border border-border rounded-lg px-4 py-3 hover:border-blue-600 transition-colors"
              >
                <Thumbnail cameraId={cam.id} name={cam.name} />
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-foreground truncate">{cam.name || cam.id}</p>
                  <div className="flex items-center gap-1.5 mt-1">
                    <StatusBadges cam={cam} />
                  </div>
                </div>
                <ChevronRight className="w-4 h-4 text-muted-foreground shrink-0" />
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
        <h3 className="text-h2 font-semibold text-foreground">Câmeras</h3>
        {!creating && !noDb && (
          <button
            onClick={() => { setCreating(true); setError(null) }}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-blue-600 hover:bg-blue-500 text-white rounded transition-colors"
          >
            <Plus className="w-3.5 h-3.5" />
            Nova câmera
          </button>
        )}
      </div>

      {noDb && (
        <p className="text-muted-foreground text-sm">Gerenciamento de câmeras requer banco de dados configurado.</p>
      )}

      {error && (
        <div className="mb-4 px-3 py-2 bg-red-900/30 border border-red-700/50 rounded text-xs text-red-400">
          {error}
        </div>
      )}

      {creating && (
        <div className="mb-4 bg-surface border border-border rounded-lg p-4">
          <p className="text-xs font-medium text-muted-foreground mb-3">Nova câmera</p>
          <CameraForm
            onSave={handleCreate}
            onCancel={() => {
              if (isNewRoute) { navigate('/settings/cameras', { replace: true }); return }
              setCreating(false); setError(null)
            }}
            saving={saving}
            prefillRtsp={prefillRTSP || undefined}
            prefillName={prefillName || undefined}
          />
        </div>
      )}

      {loading ? (
        <p className="text-muted-foreground text-sm">Carregando...</p>
      ) : cameras.length === 0 && !noDb ? (
        <p className="text-muted-foreground text-sm">Nenhuma câmera configurada.</p>
      ) : (
        <div className="flex flex-col gap-2">
          {cameras.map(cam => (
            <div
              key={cam.id}
              draggable
              onDragStart={() => { dragIdRef.current = cam.id }}
              onDragOver={e => { e.preventDefault(); setDragOverId(cam.id) }}
              onDragLeave={() => setDragOverId(null)}
              onDrop={() => handleDrop(cam.id)}
              onDragEnd={() => { dragIdRef.current = null; setDragOverId(null) }}
              className={`group bg-surface border rounded-lg px-3 py-3 flex items-center gap-3 transition-colors ${dragOverId === cam.id ? 'border-blue-500' : 'border-border hover:border-border'}`}
            >
              {/* drag handle */}
              <GripVertical className="w-4 h-4 text-muted-foreground group-hover:text-muted-foreground shrink-0 cursor-grab active:cursor-grabbing transition-colors" />

              {/* thumbnail */}
              <Thumbnail cameraId={cam.id} name={cam.name} />

              {/* info */}
              <div className="flex-1 min-w-0">
                <Link
                  to={`/settings/cameras/${cam.id}`}
                  className="text-sm font-medium text-foreground hover:text-blue-400 transition-colors truncate block"
                >
                  {cam.name || cam.id}
                </Link>
                <div className="flex items-center gap-1.5 mt-1">
                  <StatusBadges cam={cam} />
                </div>
              </div>

              {/* actions */}
              <div className="flex items-center gap-1 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
                <Link
                  to={`/settings/cameras/${cam.id}`}
                  state={{ editing: true }}
                  title="Editar"
                  className="p-1.5 text-muted-foreground hover:text-white hover:bg-accent rounded transition-colors"
                >
                  <Pencil className="w-4 h-4" />
                </Link>
                <button
                  onClick={() => setDeleteId(cam.id)}
                  title="Remover"
                  className="p-1.5 text-muted-foreground hover:text-red-400 hover:bg-red-900/20 rounded transition-colors"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      <ConfirmDialog
        open={deleteId != null}
        title="Remover câmera"
        message={`Remover câmera "${camToDelete?.name || camToDelete?.id}"?`}
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
          <span className="text-xs text-foreground">Apagar também as gravações do disco</span>
        </label>
      </ConfirmDialog>
    </SettingsLayout>
  )
}

function Thumbnail({ cameraId, name }: { cameraId: string; name?: string }) {
  return (
    <div className="w-20 h-12 shrink-0 rounded overflow-hidden bg-surface-2">
      <img
        src={`/api/cameras/${cameraId}/snapshot?token=${getToken()}`}
        alt={name || cameraId}
        className="w-full h-full object-cover"
        onError={e => { (e.currentTarget as HTMLImageElement).style.display = 'none' }}
      />
    </div>
  )
}

function StatusBadges({ cam }: { cam: Camera }) {
  return (
    <>
      {cam.motion?.enabled && (
        <span className="px-1.5 py-0.5 text-xs rounded bg-green-900/40 text-green-400 border border-green-800/50">
          motion
        </span>
      )}
      {!cam.recording_enabled && (
        <span className="px-1.5 py-0.5 text-xs rounded bg-surface-2 text-muted-foreground border border-border">
          rec off
        </span>
      )}
    </>
  )
}
