import { useEffect, useState } from 'react'
import { useParams, useLocation, useNavigate } from 'react-router-dom'
import SettingsLayout from '../../components/SettingsLayout'
import SettingsSection from '../../components/SettingsSection'
import CameraForm from '../../components/CameraForm'
import CameraSettingsTabs from '../../components/CameraSettingsTabs'
import DeviceInfoPanel from '../../components/DeviceInfoPanel'
import { type CameraFormData, type Camera, formToPayload } from '../../components/cameraFormUtils'
import { useSettings, type CameraSettings } from '../../hooks/useSettings'
import { authHeaders, getRole } from '../../auth'
import { Button } from '@/components/ui/button'

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
  const isAdmin = getRole() === 'admin'
  const location = useLocation()
  const navigate = useNavigate()
  // Edição tem URL própria (/settings/cameras/edit/:id). `editing` é DERIVADO da
  // rota — navegar p/ a URL de edição não remonta o componente, então não pode
  // depender de useState inicial; deriva direto da location.
  const editing = isAdmin && location.pathname.startsWith('/settings/cameras/edit/')
  const { settings, reload } = useSettings()
  const cam = settings?.cameras.find(c => c.id === id) as Camera | undefined
  const [stats, setStats] = useState<CameraStatsData | null>(null)

  const stopEditing = () => { setError(null); navigate(`/settings/cameras/${id}`) }
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [viewerCam, setViewerCam] = useState<CameraSettings | null>(null)
  const [viewerLoading, setViewerLoading] = useState(!isAdmin)

  useEffect(() => {
    if (!id) return
    fetch(`/api/cameras/${id}/stats`, { headers: authHeaders() })
      .then(r => r.ok ? r.json() : null)
      .then(data => { if (data) setStats(data) })
      .catch(() => {})
  }, [id])

  useEffect(() => {
    if (isAdmin || !id) return
    fetch('/api/cameras', { headers: authHeaders() })
      .then(r => r.ok ? r.json() : [])
      .then((cams: CameraSettings[]) => setViewerCam(cams.find(c => c.id === id) ?? null))
      .catch(() => {})
      .finally(() => setViewerLoading(false))
  }, [isAdmin, id])

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
      reload()
      stopEditing()
    } finally { setSaving(false) }
  }

  if (!isAdmin) {
    return (
      <SettingsLayout>
        <CameraSettingsTabs id={id!} active="detail" camName={viewerCam?.name} />
        {viewerLoading ? (
          <p className="text-muted-foreground text-sm">Carregando...</p>
        ) : !viewerCam ? (
          <p className="text-muted-foreground text-sm">Câmera não encontrada.</p>
        ) : (
          <div className="flex flex-col gap-4">
            <SettingsSection
              title="Identificação"
              fields={[
                { label: 'ID', value: viewerCam.id },
                { label: 'Nome', value: viewerCam.name },
              ]}
            />
            <SettingsSection
              title="Stream"
              fields={[
                { label: 'Codec de vídeo', value: viewerCam.video_codec || 'auto' },
                { label: 'Áudio', value: fmtHasAudio(viewerCam.has_audio) },
                { label: 'Resolução', value: fmtResolution(viewerCam.width, viewerCam.height) },
              ]}
            />
            <SettingsSection
              title="Gravação"
              fields={[
                { label: 'Gravar em disco', value: viewerCam.recording_enabled ? 'Sim' : 'Não' },
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
            <DeviceInfoPanel cameraId={id!} isAdmin={false} />
          </div>
        )}
      </SettingsLayout>
    )
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
        <p className="text-muted-foreground text-sm">Carregando...</p>
      ) : !cam ? (
        <p className="text-muted-foreground text-sm">Câmera não encontrada.</p>
      ) : editing ? (
        <CameraForm
          initial={cam}
          onSave={handleUpdate}
          onCancel={stopEditing}
          saving={saving}
        />
      ) : (
        <div className="flex flex-col gap-4">
          <div className="flex justify-end">
            <Button
              id="camera-edit"
              variant="outline"
              size="sm"
              onClick={() => navigate(`/settings/cameras/edit/${id}`)}
            >
              Editar
            </Button>
          </div>
          <SettingsSection
            title="Identificação"
            fields={[
              { label: 'Nome', value: cam.name },
              { label: 'ID', value: cam.id },
              { label: 'URL RTSP', value: cam.rtsp_url },
            ]}
          />
          <SettingsSection
            title="Vídeo"
            groups={[
              [{ label: 'Codec', value: cam.video_codec || 'auto' }, { label: 'Resolução', value: fmtResolution(cam.width, cam.height) }],
              [{ label: 'Áudio', value: fmtHasAudio(cam.has_audio) }, { label: 'Modo HLS', value: cam.hls_video_mode || 'auto' }],
              [{ label: 'Modo gravação', value: cam.record_video_mode || 'auto' }],
            ]}
          />
          <SettingsSection
            title="Transmissão ao vivo"
            groups={[
              [
                { label: 'Duração do segmento', value: cam.hls_segment_seconds != null ? `${cam.hls_segment_seconds} s` : 'padrão (2 s)' },
                { label: 'Janela de reprodução', value: cam.hls_list_size != null ? `${cam.hls_list_size} segmentos` : 'padrão (5 segmentos)' },
                { label: 'Retenção DVR', value: cam.hls_dvr_seconds ? `${cam.hls_dvr_seconds} s` : 'desativado' },
              ],
            ]}
          />
          <SettingsSection
            title="Gravação"
            groups={[
              [{ label: 'Gravar em disco', value: cam.recording_enabled ? 'Sim' : 'Não' }],
              [{ label: 'Duração do chunk', value: cam.chunk_duration }, { label: 'Intervalo de reconexão', value: cam.reconnect_interval }],
            ]}
          />
          <SettingsSection
            title="Estatísticas"
            groups={
              stats == null
                ? [[{ label: 'Carregando...', value: '' }]]
                : [
                    [{ label: 'Total gravado', value: fmtDuration(stats.total_seconds) }, { label: 'Segmentos MP4', value: String(stats.total_chunks) }],
                    [{ label: 'Espaço em disco', value: fmtBytes(stats.total_bytes) }, { label: 'Eventos de movimento', value: String(stats.total_motion_events) }],
                  ]
            }
          />
          <DeviceInfoPanel cameraId={id!} isAdmin={isAdmin} />
        </div>
      )}
    </SettingsLayout>
  )
}
