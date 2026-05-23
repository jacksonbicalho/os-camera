import { useState, useEffect, useCallback, useRef } from 'react'
import { Link, useNavigate, useLocation } from 'react-router-dom'
import SettingsLayout from '../../components/SettingsLayout'
import ConfirmDialog from '../../components/ConfirmDialog'
import CameraForm from '../../components/CameraForm'
import { type Camera, type CameraFormData, formToPayload } from '../../components/cameraFormUtils'
import { authHeaders, clearToken, getRole, getToken } from '../../auth'

export default function CamerasSettingsPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const isAdmin = getRole() === 'admin'
  const isNewRoute = location.pathname === '/settings/cameras/new'
  const [cameras, setCameras] = useState<Camera[]>([])
  const [loading, setLoading] = useState(isAdmin)
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
                className="flex items-center gap-4 bg-gray-900 border border-gray-800 rounded-lg px-4 py-3 hover:border-blue-600 transition-colors"
              >
                <div className="w-24 h-14 shrink-0 rounded overflow-hidden bg-gray-800">
                  <img
                    src={`/api/cameras/${cam.id}/snapshot?token=${getToken()}`}
                    alt={cam.name || cam.id}
                    className="w-full h-full object-cover"
                    onError={e => { (e.currentTarget as HTMLImageElement).style.display = 'none' }}
                  />
                </div>
                <span className="flex-1 text-sm font-mono text-gray-200 truncate">{cam.name || cam.id}</span>
                <svg className="w-4 h-4 text-gray-500 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
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
        <h2 className="text-lg font-semibold text-gray-200">Câmeras</h2>
        {!creating && !noDb && (
          <button
            onClick={() => { setCreating(true); setError(null) }}
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
            <div
              key={cam.id}
              draggable
              onDragStart={() => { dragIdRef.current = cam.id }}
              onDragOver={e => { e.preventDefault(); setDragOverId(cam.id) }}
              onDragLeave={() => setDragOverId(null)}
              onDrop={() => handleDrop(cam.id)}
              onDragEnd={() => { dragIdRef.current = null; setDragOverId(null) }}
              className={`bg-gray-900 border rounded-lg px-4 py-3 flex items-center gap-4 transition-colors ${dragOverId === cam.id ? 'border-blue-500' : 'border-gray-800'}`}
            >
              <svg className="w-4 h-4 text-gray-600 shrink-0 cursor-grab active:cursor-grabbing" fill="currentColor" viewBox="0 0 20 20">
                <path d="M7 4a1 1 0 1 0 0-2 1 1 0 0 0 0 2zm6 0a1 1 0 1 0 0-2 1 1 0 0 0 0 2zM7 9a1 1 0 1 0 0-2 1 1 0 0 0 0 2zm6 0a1 1 0 1 0 0-2 1 1 0 0 0 0 2zm-6 5a1 1 0 1 0 0-2 1 1 0 0 0 0 2zm6 0a1 1 0 1 0 0-2 1 1 0 0 0 0 2z" />
              </svg>
              <div className="w-24 h-14 shrink-0 rounded overflow-hidden bg-gray-800">
                <img
                  src={`/api/cameras/${cam.id}/snapshot?token=${getToken()}`}
                  alt={cam.name || cam.id}
                  className="w-full h-full object-cover"
                  onError={e => { (e.currentTarget as HTMLImageElement).style.display = 'none' }}
                />
              </div>
              <div className="flex-1 flex items-center justify-between gap-3 min-w-0">
                <div className="flex items-center gap-3 min-w-0">
                  <Link
                    to={`/settings/cameras/${cam.id}`}
                    className="text-sm font-mono text-gray-200 hover:text-blue-400 transition-colors truncate"
                  >
                    {cam.name || cam.id}
                  </Link>
                  {cam.motion?.enabled && (
                    <span className="px-2 py-0.5 text-xs rounded-full bg-green-900/40 text-green-400 border border-green-700/40 shrink-0">
                      motion
                    </span>
                  )}
                  {!cam.recording_enabled && (
                    <span className="px-2 py-0.5 text-xs rounded-full bg-gray-800 text-gray-500 border border-gray-700 shrink-0">
                      rec off
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-1 shrink-0">
                  <Link
                    to={`/settings/cameras/${cam.id}`}
                    state={{ editing: true }}
                    className="px-3 py-1 text-xs text-gray-400 hover:text-white border border-gray-700 rounded transition-colors"
                  >
                    Editar
                  </Link>
                  <button
                    onClick={() => setDeleteId(cam.id)}
                    className="px-3 py-1 text-xs text-red-500 hover:text-red-400 border border-gray-700 rounded transition-colors"
                  >
                    Remover
                  </button>
                </div>
              </div>
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
