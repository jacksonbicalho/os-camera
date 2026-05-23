import { useEffect, useState } from 'react'
import { useParams, useLocation } from 'react-router-dom'
import SettingsLayout from '../../components/SettingsLayout'
import SettingsSection from '../../components/SettingsSection'
import CameraForm from '../../components/CameraForm'
import CameraSettingsTabs from '../../components/CameraSettingsTabs'
import { type CameraFormData, type Camera, formToPayload } from '../../components/cameraFormUtils'
import { useSettings } from '../../hooks/useSettings'
import { authHeaders } from '../../auth'

function fmtHasAudio(v: boolean | null): string {
  if (v === null) return 'auto'
  return v ? 'sim' : 'não'
}

function fmtResolution(w: number, h: number): string {
  if (w === 0 && h === 0) return 'auto'
  return `${w} × ${h}`
}

function fmtBytes(b: number): string {
  if (b === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(b) / Math.log(1024))
  return `${(b / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}

function fmtDuration(seconds: number): string {
  if (seconds < 60) return `${Math.round(seconds)}s`
  if (seconds < 3600) return `${Math.round(seconds / 60)}min`
  const h = Math.floor(seconds / 3600)
  const m = Math.round((seconds % 3600) / 60)
  return m > 0 ? `${h}h ${m}min` : `${h}h`
}

interface CameraStatsData {
  total_bytes: number
  total_chunks: number
  total_seconds: number
  total_motion_events: number
}

export default function CameraDetailSettingsPage() {
  const { id } = useParams<{ id: string }>()
  const location = useLocation()
  const startEditing = (location.state as { editing?: boolean } | null)?.editing ?? false
  const { settings, reload } = useSettings(`/settings/cameras/${id}`)
  const cam = settings?.cameras.find(c => c.id === id) as Camera | undefined
  const [stats, setStats] = useState<CameraStatsData | null>(null)
  const [editing, setEditing] = useState(startEditing)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!id) return
    fetch(`/api/cameras/${id}/stats`, { headers: authHeaders() })
      .then(r => r.ok ? r.json() : null)
      .then(data => { if (data) setStats(data) })
      .catch(() => {})
  }, [id])

  const handleUpdate = async (data: CameraFormData) => {
    if (!id) return
    setSaving(true); setError(null)
    try {
      const res = await fetch(`/api/settings/cameras/${id}`, {
        method: 'PUT',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify(formToPayload(data)),
      })
      if (!res.ok) { setError((await res.text()).trim() || 'Erro ao atualizar câmera'); return }
      setEditing(false)
      reload()
    } finally { setSaving(false) }
  }

  return (
    <SettingsLayout>
      <CameraSettingsTabs id={id!} active="detail" camName={cam?.name} />

      {error && (
        <div className="mb-4 px-3 py-2 bg-red-900/30 border border-red-700/50 rounded text-xs text-red-400">
          {error}
        </div>
      )}

      {!settings ? (
        <p className="text-gray-500 text-sm">Carregando...</p>
      ) : !cam ? (
        <p className="text-gray-500 text-sm">Câmera não encontrada.</p>
      ) : editing ? (
        <CameraForm
          initial={cam}
          onSave={handleUpdate}
          onCancel={() => { setEditing(false); setError(null) }}
          saving={saving}
        />
      ) : (
        <div className="flex flex-col gap-4">
          <div className="flex justify-end">
            <button
              onClick={() => { setEditing(true); setError(null) }}
              className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 border border-gray-700 text-gray-300 hover:text-white rounded transition-colors"
            >
              Editar
            </button>
          </div>
          <SettingsSection
            title="Identificação"
            fields={[
              { label: 'ID', value: cam.id },
              { label: 'Nome', value: cam.name },
              { label: 'URL RTSP', value: cam.rtsp_url },
            ]}
          />
          <SettingsSection
            title="Stream"
            fields={[
              { label: 'Codec de vídeo', value: cam.video_codec || 'auto' },
              { label: 'Modo HLS', value: cam.hls_video_mode || 'auto' },
              { label: 'Modo gravação', value: cam.record_video_mode || 'auto' },
              { label: 'Áudio', value: fmtHasAudio(cam.has_audio) },
              { label: 'Resolução', value: fmtResolution(cam.width, cam.height) },
              {
                label: 'Segmento HLS',
                value: cam.hls_segment_seconds != null ? `${cam.hls_segment_seconds} s` : 'padrão (2 s)',
              },
              {
                label: 'Janela HLS',
                value: cam.hls_list_size != null ? `${cam.hls_list_size} segmentos` : 'padrão (5 segmentos)',
              },
            ]}
          />
          <SettingsSection
            title="Gravação"
            fields={[
              { label: 'Gravar em disco', value: cam.recording_enabled ? 'Sim' : 'Não' },
              { label: 'Duração do chunk', value: cam.chunk_duration },
              { label: 'Intervalo de reconexão', value: cam.reconnect_interval },
            ]}
          />
          <SettingsSection
            title="Estatísticas"
            fields={
              stats == null
                ? [{ label: 'Carregando...', value: '' }]
                : [
                    { label: 'Total gravado', value: fmtDuration(stats.total_seconds) },
                    { label: 'Segmentos MP4', value: String(stats.total_chunks) },
                    { label: 'Espaço em disco', value: fmtBytes(stats.total_bytes) },
                    { label: 'Eventos de movimento', value: String(stats.total_motion_events) },
                  ]
            }
          />
        </div>
      )}
    </SettingsLayout>
  )
}
