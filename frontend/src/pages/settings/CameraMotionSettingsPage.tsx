import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import SettingsLayout from '../../components/SettingsLayout'
import SettingsSection from '../../components/SettingsSection'
import MotionScoreChart from '../../components/MotionScoreChart'
import CameraSettingsTabs from '../../components/CameraSettingsTabs'
import { useSettings, type CameraSettings } from '../../hooks/useSettings'
import { useMotionPeak, type MotionDailyPeak } from '../../hooks/useMotionPeak'
import { emptyForm, formToPayload, type Camera, type CameraFormData } from '../../components/cameraFormUtils'
import { authHeaders, getRole } from '../../auth'

function formatScore(v: number): string {
  if (v <= 0) return '—'
  if (v >= 1) return v.toFixed(2)
  const decimals = Math.max(2, -Math.floor(Math.log10(v)) + 1)
  return v.toFixed(decimals)
}

function ratioLabel(peak: number, threshold: number): React.ReactNode {
  if (peak === 0 || threshold === 0) return '—'
  const ratio = peak / threshold
  const ratioStr = `${formatScore(peak)} / ${formatScore(threshold)} = ${ratio.toFixed(2)}×`

  let hint: string
  if (ratio >= 1) {
    hint = 'Pico ultrapassou o limiar — eventos de movimento foram registrados hoje.'
  } else if (ratio >= 0.5) {
    hint = 'Pico próximo ao limiar — considere reduzir o limiar para capturar este nível de movimento.'
  } else {
    hint = 'Pico bem abaixo do limiar — nenhum evento foi disparado hoje.'
  }

  return (
    <span>
      {ratioStr}
      <span className="block mt-1 text-xs text-gray-500 font-sans">{hint}</span>
    </span>
  )
}

function RatioGuide({ peak, threshold }: { peak: number; threshold: number }) {
  const ratio = threshold > 0 ? peak / threshold : 0
  const zone: 'high' | 'mid' | 'low' = ratio >= 1 ? 'high' : ratio >= 0.5 ? 'mid' : 'low'

  const rows: Array<{
    id: 'high' | 'mid' | 'low'
    range: string
    color: string
    example: string
    suggestion: string
  }> = [
    {
      id: 'high',
      range: '≥ 1×',
      color: 'text-green-400',
      example: zone === 'high' ? `${ratio.toFixed(2)}×` : '1.50×',
      suggestion: zone === 'high'
        ? `Limiar ${formatScore(threshold)} funcionando. Se houver falsos positivos, aumente para ~${formatScore(threshold * 2)}.`
        : 'Pico ultrapassou o limiar — eventos registrados. Aumente o limiar se houver falsos positivos.',
    },
    {
      id: 'mid',
      range: '0.5× – 1×',
      color: 'text-yellow-400',
      example: zone === 'mid' ? `${ratio.toFixed(2)}×` : '0.75×',
      suggestion: zone === 'mid'
        ? `Pico (${formatScore(peak)}) próximo ao limiar (${formatScore(threshold)}). Reduza para ~${formatScore(peak * 0.8)} para capturar este movimento.`
        : 'Próximo ao limiar. Reduza o limiar para capturar este nível de movimento.',
    },
    {
      id: 'low',
      range: '< 0.5×',
      color: 'text-gray-500',
      example: zone === 'low' ? `${ratio.toFixed(2)}×` : '0.41×',
      suggestion: zone === 'low'
        ? `Pico (${formatScore(peak)}) bem abaixo do limiar (${formatScore(threshold)}). Para detectar este nível, reduza para ~${formatScore(peak * 1.5)}.`
        : 'Bem abaixo do limiar — nenhum evento disparado.',
    },
  ]

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-lg px-5 py-4">
      <p className="text-xs font-medium text-gray-400 uppercase tracking-wider mb-3">Como interpretar a relação</p>
      <table className="w-full text-xs border-collapse">
        <thead>
          <tr className="border-b border-gray-800">
            <th className="text-left text-gray-500 font-medium pb-2 pr-4">Situação</th>
            <th className="text-left text-gray-500 font-medium pb-2 pr-4">Hoje</th>
            <th className="text-left text-gray-500 font-medium pb-2">Sugestão</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-800">
          {rows.map(row => {
            const active = row.id === zone
            return (
              <tr key={row.id} className={active ? 'bg-gray-800/50' : 'opacity-40'}>
                <td className={`py-2 pr-4 font-mono whitespace-nowrap ${row.color}`}>{row.range}</td>
                <td className={`py-2 pr-4 font-mono whitespace-nowrap ${active ? 'text-white' : 'text-gray-400'}`}>
                  {row.example}
                </td>
                <td className={`py-2 ${active ? 'text-gray-300' : 'text-gray-500'}`}>{row.suggestion}</td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

function MotionReadOnly({ cam, id, peak }: { cam: CameraSettings | null; id: string; peak: MotionDailyPeak | null }) {
  if (!cam) return <p className="text-gray-500 text-sm">Câmera não encontrada.</p>
  const motion = cam.motion
  if (!motion?.enabled) {
    return (
      <SettingsSection
        title="Detecção de movimento"
        fields={[{ label: 'Status', value: 'Desabilitado' }]}
      />
    )
  }
  return (
    <div className="flex flex-col gap-4">
      <SettingsSection
        title="Configuração"
        fields={[
          { label: 'Status', value: 'Habilitado' },
          { label: 'Limiar', value: formatScore(motion.threshold) },
          { label: 'FPS de análise', value: motion.fps },
          { label: 'Cooldown (s)', value: motion.cooldown_seconds },
        ]}
      />
      <div className="bg-gray-900 border border-gray-800 rounded-lg px-5 py-4">
        <p className="text-xs font-medium text-gray-400 mb-3">Score em tempo real</p>
        <MotionScoreChart cameraId={id} threshold={motion.threshold} />
      </div>
      {peak !== null && (
        <SettingsSection
          title="Hoje"
          fields={[
            { label: 'Pico de score bruto', value: formatScore(peak.peak_raw_score) },
            { label: 'Limiar configurado', value: formatScore(motion.threshold) },
          ]}
        />
      )}
    </div>
  )
}

const inputClass = 'w-full bg-gray-800 border border-gray-700 rounded px-2 py-1 text-xs text-gray-200 focus:outline-none focus:border-blue-500'
const labelClass = 'block text-xs text-gray-400 mb-1'

interface MotionFormContentProps {
  cam: Camera
  id: string
  peak: { peak_raw_score: number } | null
  reload: () => void
}



function MotionFormContent({ cam, id, peak, reload }: MotionFormContentProps) {
  const [form, setForm] = useState<CameraFormData>(() => emptyForm(cam))
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)
  const [saveCount, setSaveCount] = useState(0)

  const set = (field: keyof CameraFormData, value: string | boolean | number) =>
    setForm(prev => ({ ...prev, [field]: value }))

  const effectiveThreshold = parseFloat(form.motion_threshold) || 0
  const streamW = cam.width ?? 0
  const streamH = cam.height ?? 0
  const previewW = form.motion_capture_auto
    ? (streamW > 0 ? Math.round(streamW / 4) : null)
    : (streamW > 0 ? Math.round(streamW * form.motion_capture_pct / 100) : null)
  const previewH = form.motion_capture_auto
    ? (streamH > 0 ? Math.round(streamH / 4) : null)
    : (streamH > 0 ? Math.round(streamH * form.motion_capture_pct / 100) : null)

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    setSaving(true); setError(null); setSaved(false)
    try {
      const res = await fetch(`/api/settings/cameras/${id}`, {
        method: 'PUT',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify(formToPayload(form)),
      })
      if (!res.ok) { setError((await res.text()).trim() || 'Erro ao salvar'); return }
      setSaved(true)
      setSaveCount(c => c + 1)
      reload()
      setTimeout(() => setSaved(false), 2000)
    } finally { setSaving(false) }
  }

  return (
    <form onSubmit={handleSave} className="flex flex-col gap-4">

      <div className="bg-gray-900 border border-gray-800 rounded-lg px-5 py-4">
        <div className="flex items-center gap-2">
          <input
            type="checkbox"
            id="motion_enabled"
            checked={form.motion_enabled}
            onChange={e => set('motion_enabled', e.target.checked)}
            className="accent-blue-500"
          />
          <label htmlFor="motion_enabled" className="text-xs text-gray-400 cursor-pointer">Habilitado</label>
        </div>
      </div>

      {form.motion_enabled && (
        <>
          <div className="bg-gray-900 border border-gray-800 rounded-lg px-5 py-4">
            <p className="text-xs font-medium text-gray-400 mb-3">Score em tempo real</p>
            <MotionScoreChart key={saveCount} cameraId={id} threshold={effectiveThreshold} />
          </div>

          {peak !== null && (
            <>
              <SettingsSection
                title="Hoje"
                fields={[
                  { label: 'Pico de score bruto', value: formatScore(peak.peak_raw_score) },
                  { label: 'Limiar configurado', value: effectiveThreshold },
                  { label: 'Relação pico / limiar', value: ratioLabel(peak.peak_raw_score, effectiveThreshold) },
                ]}
              />
              <RatioGuide peak={peak.peak_raw_score} threshold={effectiveThreshold} />
            </>
          )}

          <div className="bg-gray-900 border border-gray-800 rounded-lg px-5 py-4 flex flex-col gap-4">
            <p className="text-xs font-medium text-gray-400 uppercase tracking-wider">Configuração</p>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label className={labelClass}>Limiar</label>
                <input type="number" step="0.001" min="0.001" max="1" value={form.motion_threshold} onChange={e => set('motion_threshold', e.target.value)} className={inputClass} />
                <p className="text-xs text-gray-600 mt-0.5">0.001 – 1.0 · quanto menor, mais sensível</p>
              </div>
              <div>
                <label className={labelClass}>FPS de análise</label>
                <input type="number" min="1" max="30" value={form.motion_fps} onChange={e => set('motion_fps', e.target.value)} className={inputClass} />
                <p className="text-xs text-gray-600 mt-0.5">1 – 30 fps · padrão: 2</p>
              </div>
              <div className="sm:col-span-2 grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div>
                  <label className={labelClass}>Cooldown (segundos)</label>
                  <input type="number" min="0" value={form.motion_cooldown} onChange={e => set('motion_cooldown', e.target.value)} className={inputClass} />
                  <p className="text-xs text-gray-600 mt-0.5">Tempo mínimo entre eventos · 0 = sem cooldown</p>
                </div>
                <div>
                  <label className={labelClass}>Segundos antes do evento</label>
                  <input type="number" min="0" max="300" value={form.motion_playback_lead} onChange={e => set('motion_playback_lead', e.target.value)} className={inputClass} />
                  <p className="text-xs text-gray-600 mt-0.5">0 – 300 s · recua o player antes do instante detectado</p>
                </div>
                <div>
                  <label className={labelClass}>Segundos após o evento</label>
                  <input type="number" min="0" max="300" value={form.motion_playback_trail} onChange={e => set('motion_playback_trail', e.target.value)} className={inputClass} />
                  <p className="text-xs text-gray-600 mt-0.5">0 – 300 s · preserva chunks gravados após o evento</p>
                </div>
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
            </div>
          </div>
        </>
      )}

      {error && (
        <div className="px-3 py-2 bg-red-900/30 border border-red-700/50 rounded text-xs text-red-400">
          {error}
        </div>
      )}

      <div className="flex items-center gap-3">
        <button
          type="submit"
          disabled={saving}
          className="px-4 py-1.5 text-xs bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded transition-colors"
        >
          {saving ? 'Salvando...' : 'Salvar'}
        </button>
        {saved && <span className="text-xs text-green-400">Salvo</span>}
      </div>

    </form>
  )
}

export default function CameraMotionSettingsPage() {
  const { id } = useParams<{ id: string }>()
  const isAdmin = getRole() === 'admin'
  const { settings, reload } = useSettings()
  const peak = useMotionPeak(id, `/settings/cameras/${id}/motion`)
  const cam = settings?.cameras.find(c => c.id === id) as Camera | undefined

  const [viewerCam, setViewerCam] = useState<CameraSettings | null>(null)
  const [viewerLoading, setViewerLoading] = useState(!isAdmin)

  useEffect(() => {
    if (isAdmin || !id) return
    fetch('/api/cameras', { headers: authHeaders() })
      .then(r => r.ok ? r.json() : [])
      .then((cams: CameraSettings[]) => setViewerCam(cams.find(c => c.id === id) ?? null))
      .catch(() => {})
      .finally(() => setViewerLoading(false))
  }, [isAdmin, id])

  return (
    <SettingsLayout>
      <CameraSettingsTabs id={id!} active="motion" camName={isAdmin ? cam?.name : viewerCam?.name} />
      {!isAdmin ? (
        viewerLoading ? (
          <p className="text-gray-500 text-sm">Carregando...</p>
        ) : (
          <MotionReadOnly cam={viewerCam} id={id!} peak={peak} />
        )
      ) : !settings ? (
        <p className="text-gray-500 text-sm">Carregando...</p>
      ) : !cam ? (
        <p className="text-gray-500 text-sm">Câmera não encontrada.</p>
      ) : (
        <MotionFormContent cam={cam} id={id!} peak={peak} reload={reload} />
      )}
    </SettingsLayout>
  )
}
