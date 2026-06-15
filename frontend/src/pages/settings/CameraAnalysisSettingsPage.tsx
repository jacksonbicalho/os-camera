import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import SettingsLayout from '../../components/SettingsLayout'
import CameraSettingsTabs from '../../components/CameraSettingsTabs'
import { useSettings, type CameraSettings } from '../../hooks/useSettings'
import { authHeaders } from '../../auth'

export default function CameraAnalysisSettingsPage() {
  const { id } = useParams<{ id: string }>()
  const { settings } = useSettings()
  const cam = settings?.cameras?.find((c: CameraSettings) => c.id === id)

  const [enabled, setEnabled] = useState(true)
  const [globalEnabled, setGlobalEnabled] = useState(true)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!id) return
    Promise.all([
      fetch(`/api/settings/cameras/${id}/analysis`, { headers: authHeaders() }).then(r => r.json()),
      fetch('/api/settings/analysis', { headers: authHeaders() }).then(r => r.json()),
    ])
      .then(([cam, global]) => {
        setEnabled(cam.enabled ?? true)
        setGlobalEnabled(global.enabled ?? true)
      })
      .catch(() => setError('Falha ao carregar configuração'))
  }, [id])

  async function handleSave() {
    setError('')
    setSaving(true)
    try {
      const res = await fetch(`/api/settings/cameras/${id}/analysis`, {
        method: 'PUT',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled }),
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
      <CameraSettingsTabs id={id!} active="analysis" camName={cam?.name} />

      <div className="space-y-6">
        <div className="bg-surface-2 rounded-lg border border-border divide-y divide-border">
          {!globalEnabled && (
            <div className="px-4 py-3 bg-yellow-900/20 border-b border-yellow-700/30 flex items-start gap-2">
              <span className="text-yellow-400 text-xs mt-0.5">⚠</span>
              <p className="text-xs text-yellow-400">
                A análise global está desativada.{' '}
                <a href="/settings/analysis" className="underline hover:text-yellow-300">Ativar em Configurações → Análise de vídeo</a>{' '}
                para que esta câmera seja processada.
              </p>
            </div>
          )}
          <div className="p-4 flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-foreground">Análise de objetos</p>
              <p className="text-xs text-muted-foreground mt-0.5">
                Ativar detecção YOLO nas gravações desta câmera.
              </p>
            </div>
            <button
              type="button"
              onClick={() => { if (globalEnabled) setEnabled(v => !v) }}
              disabled={!globalEnabled}
              className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${enabled && globalEnabled ? 'bg-blue-600' : 'bg-surface-2'} ${!globalEnabled ? 'opacity-40 cursor-not-allowed' : ''}`}
            >
              <span className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${enabled ? 'translate-x-6' : 'translate-x-1'}`} />
            </button>
          </div>

          <div className="p-4 flex items-center justify-between">
            {error && <p className="text-sm text-red-400">{error}</p>}
            {saved && <p className="text-sm text-green-400">Salvo</p>}
            {!error && !saved && <span />}
            <button
              onClick={handleSave}
              disabled={saving}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white text-sm rounded transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
            >
              {saving ? 'Salvando...' : 'Salvar'}
            </button>
          </div>
        </div>
      </div>
    </SettingsLayout>
  )
}
