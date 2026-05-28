import { useEffect, useRef, useState } from 'react'
import SettingsLayout from '../../components/SettingsLayout'
import { useSettings } from '../../hooks/useSettings'
import { authHeaders } from '../../auth'

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
  const [annCount, setAnnCount] = useState<number | null>(null)
  const [ftJobID, setFtJobID] = useState<string | null>(() => localStorage.getItem('ft_job_id'))
  const [ftStatus, setFtStatus] = useState<{ status: string; epoch: number; total_epochs: number; error: string } | null>(null)
  const [ftError, setFtError] = useState('')
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    fetch('/api/settings/analysis', { headers: authHeaders() })
      .then(r => r.json())
      .then(setCfg)
      .catch(() => setError('Falha ao carregar configurações'))
    fetch('/api/settings/analysis/annotation-count', { headers: authHeaders() })
      .then(r => r.ok ? r.json() : null)
      .then(d => d && setAnnCount(d.count))
      .catch(() => {})
  }, [])

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

  async function handleStartFinetune() {
    setFtError('')
    setFtStatus(null)
    setFtJobID(null)
    localStorage.removeItem('ft_job_id')
    const res = await fetch('/api/settings/analysis/finetune', {
      method: 'POST',
      headers: authHeaders(),
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

        <div className="bg-gray-800 rounded-lg border border-gray-700 divide-y divide-gray-700">
          <div className="p-4">
            <h4 className="text-sm font-semibold text-gray-200 mb-1">Fine-tuning</h4>
            <p className="text-xs text-gray-400">
              Treina um modelo personalizado usando os snapshots que você anotou nos eventos de movimento.
              O modelo gerado (<code className="bg-gray-700 px-1 rounded">custom.pt</code>) fica disponível no seletor acima.
            </p>
          </div>

          <div className="p-4 flex items-center justify-between">
            <div>
              <p className="text-sm text-gray-300">
                {annCount === null ? '…' : annCount} anotação{annCount !== 1 ? 'ões' : ''} disponível{annCount !== 1 ? 'eis' : ''}
              </p>
              <p className="text-xs text-gray-500 mt-0.5">
                Abra um evento de movimento e clique em "Anotar" para adicionar bounding boxes
              </p>
            </div>
            <button
              type="button"
              disabled={!annCount || annCount === 0 || (ftStatus?.status === 'running' || ftStatus?.status === 'pending')}
              onClick={handleStartFinetune}
              className="px-4 py-2 bg-violet-600 hover:bg-violet-500 text-white text-sm rounded transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
            >
              Treinar agora
            </button>
          </div>

          {ftStatus && (
            <div className="p-4 space-y-2">
              {(ftStatus.status === 'running' || ftStatus.status === 'pending') && (
                <>
                  <div className="flex justify-between text-xs text-gray-400">
                    <span>{ftStatus.status === 'pending' ? 'Iniciando…' : `Época ${ftStatus.epoch} / ${ftStatus.total_epochs}`}</span>
                    <span>{ftStatus.status === 'running' ? `${Math.round((ftStatus.epoch / ftStatus.total_epochs) * 100)}%` : ''}</span>
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
      </div>
    </SettingsLayout>
  )
}
