import { useState } from 'react'
import { type Camera, type CameraFormData, RESOLUTIONS, emptyForm, decodeResolution } from './cameraFormUtils'

interface CameraFormProps {
  initial?: Camera
  onSave: (data: CameraFormData) => Promise<void>
  onCancel: () => void
  saving: boolean
}

export default function CameraForm({ initial, onSave, onCancel, saving }: CameraFormProps) {
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
          <p className="text-xs text-gray-600 mt-0.5">ex: 30s, 5m, 1h</p>
        </div>
        <div>
          <label className={labelClass}>Intervalo de reconexão</label>
          <input value={form.reconnect_interval} onChange={e => set('reconnect_interval', e.target.value)} className={inputClass} placeholder="30s" />
          <p className="text-xs text-gray-600 mt-0.5">ex: 10s, 1m, 5m</p>
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
        <div>
          <label className={labelClass}>Modo de gravação</label>
          <select value={form.record_video_mode} onChange={e => set('record_video_mode', e.target.value)} className={inputClass}>
            <option value="auto">Auto (transcodifica HEVC → H.264)</option>
            <option value="h264">H.264 (sempre transcodifica)</option>
            <option value="copy">Cópia (sem transcodificação)</option>
          </select>
        </div>

        <div>
          <label className={labelClass}>Duração do segmento HLS (s)</label>
          <select
            value={form.hls_segment_seconds}
            onChange={e => set('hls_segment_seconds', e.target.value)}
            disabled={form.hls_segment_seconds_default}
            className={`${inputClass} disabled:opacity-40 disabled:cursor-not-allowed`}
          >
            <option value="1">1 s</option>
            <option value="2">2 s</option>
            <option value="4">4 s</option>
          </select>
          <label className="flex items-center gap-1.5 mt-1 cursor-pointer">
            <input
              type="checkbox"
              checked={form.hls_segment_seconds_default}
              onChange={e => set('hls_segment_seconds_default', e.target.checked)}
              className="accent-blue-500"
            />
            <span className="text-xs text-gray-500">Usar valor padrão (2 s)</span>
          </label>
        </div>

        <div>
          <label className={labelClass}>Tamanho da janela HLS</label>
          <select
            value={form.hls_list_size}
            onChange={e => set('hls_list_size', e.target.value)}
            disabled={form.hls_list_size_default}
            className={`${inputClass} disabled:opacity-40 disabled:cursor-not-allowed`}
          >
            <option value="2">2 segmentos</option>
            <option value="3">3 segmentos</option>
            <option value="5">5 segmentos</option>
            <option value="10">10 segmentos</option>
          </select>
          <label className="flex items-center gap-1.5 mt-1 cursor-pointer">
            <input
              type="checkbox"
              checked={form.hls_list_size_default}
              onChange={e => set('hls_list_size_default', e.target.checked)}
              className="accent-blue-500"
            />
            <span className="text-xs text-gray-500">Usar valor padrão (5 segmentos)</span>
          </label>
          <p className="text-xs text-gray-600 mt-0.5">Segmento menor + janela menor = menos latência ao vivo</p>
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
                <label className={labelClass}>Limiar</label>
                <input type="number" step="0.001" min="0.001" max="1" value={form.motion_threshold} onChange={e => set('motion_threshold', e.target.value)} className={inputClass} />
                <p className="text-xs text-gray-600 mt-0.5">0.001 – 1.0 · quanto menor, mais sensível</p>
              </div>
              <div>
                <label className={labelClass}>FPS de análise</label>
                <input type="number" min="1" max="30" value={form.motion_fps} onChange={e => set('motion_fps', e.target.value)} className={inputClass} />
                <p className="text-xs text-gray-600 mt-0.5">1 – 30 fps · padrão: 2</p>
              </div>
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
