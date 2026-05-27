import { useEffect, useState } from 'react'
import SettingsLayout from '../../components/SettingsLayout'
import { useSettings } from '../../hooks/useSettings'
import { authHeaders } from '../../auth'

interface AnalysisConfig {
  enabled: boolean
  service_url: string
  model: string
  confidence_threshold: number
}

const MODELS = ['yolov8n', 'yolov8s', 'yolov8m', 'yolov8l', 'yolov8x']

export default function AnalysisSettingsPage() {
  useSettings('/login')
  const [cfg, setCfg] = useState<AnalysisConfig>({
    enabled: false,
    service_url: '',
    model: 'yolov8n',
    confidence_threshold: 0.4,
  })
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    fetch('/api/settings/analysis', { headers: authHeaders() })
      .then(r => r.json())
      .then(setCfg)
      .catch(() => setError('Falha ao carregar configurações'))
  }, [])

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    setError('')
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
                  {MODELS.map(m => (
                    <option key={m} value={m}>{m}</option>
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
              className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white text-sm rounded transition-colors"
            >
              Salvar
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
      </div>
    </SettingsLayout>
  )
}
