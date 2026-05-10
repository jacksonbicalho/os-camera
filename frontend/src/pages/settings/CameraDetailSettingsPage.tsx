import { Link, useParams } from 'react-router-dom'
import SettingsLayout from '../../components/SettingsLayout'
import SettingsSection from '../../components/SettingsSection'
import { useSettings } from '../../hooks/useSettings'

function fmtHasAudio(v: boolean | null): string {
  if (v === null) return 'auto'
  return v ? 'sim' : 'não'
}

function fmtResolution(w: number, h: number): string {
  if (w === 0 && h === 0) return 'auto'
  return `${w} × ${h}`
}

export default function CameraDetailSettingsPage() {
  const { id } = useParams<{ id: string }>()
  const settings = useSettings(`/settings/cameras/${id}`)
  const cam = settings?.cameras.find(c => c.id === id)

  return (
    <SettingsLayout>
      <h2 className="text-lg font-semibold text-gray-200 mb-6">{id}</h2>
      {!settings ? (
        <p className="text-gray-500 text-sm">Carregando...</p>
      ) : !cam ? (
        <p className="text-gray-500 text-sm">Câmera não encontrada.</p>
      ) : (
        <div className="flex flex-col gap-4">
          <SettingsSection
            title="Identificação"
            fields={[
              { label: 'ID', value: cam.id },
              { label: 'URL RTSP', value: cam.rtsp_url },
            ]}
          />
          <SettingsSection
            title="Stream"
            fields={[
              { label: 'Codec de vídeo', value: cam.video_codec || 'auto' },
              { label: 'Áudio', value: fmtHasAudio(cam.has_audio) },
              { label: 'Resolução', value: fmtResolution(cam.width, cam.height) },
            ]}
          />
          <SettingsSection
            title="Gravação"
            fields={[
              { label: 'Duração do chunk', value: cam.chunk_duration === '0s' ? `herda global (${settings.defaults.chunk_duration})` : cam.chunk_duration },
              { label: 'Intervalo de reconexão', value: cam.reconnect_interval === '0s' ? `herda global (${settings.defaults.reconnect_interval})` : cam.reconnect_interval },
            ]}
          />
          <Link
            to={`/settings/cameras/${id}/motion`}
            className="bg-gray-900 border border-gray-800 rounded-lg px-5 py-4 flex items-center justify-between hover:border-gray-700 hover:bg-gray-800/50 transition-colors group"
          >
            <div>
              <p className="text-sm font-medium text-gray-300">Detecção de movimento</p>
              <p className="text-xs text-gray-500 mt-0.5">
                {cam.motion ? 'configuração override ativa' : 'herda configuração global'}
              </p>
            </div>
            <span className="text-sm text-blue-400 group-hover:text-blue-300 transition-colors">Ver detalhes →</span>
          </Link>
          <Link
            to={`/settings/cameras/${id}/zones`}
            className="bg-gray-900 border border-gray-800 rounded-lg px-5 py-4 flex items-center justify-between hover:border-gray-700 hover:bg-gray-800/50 transition-colors group"
          >
            <div>
              <p className="text-sm font-medium text-gray-300">Zonas de exclusão</p>
              <p className="text-xs text-gray-500 mt-0.5">
                Áreas do frame ignoradas na detecção de movimento
              </p>
            </div>
            <span className="text-sm text-blue-400 group-hover:text-blue-300 transition-colors">Editar →</span>
          </Link>
        </div>
      )}
    </SettingsLayout>
  )
}
