import { useState } from 'react'
import { type Camera, type CameraFormData, RESOLUTIONS, emptyForm } from './cameraFormUtils'
import { Button } from '@/components/ui/button'
import { authHeaders } from '../auth'

interface CameraFormProps {
  initial?: Camera
  prefillRtsp?: string
  prefillName?: string
  onSave: (data: CameraFormData) => Promise<void>
  onCancel: () => void
  saving: boolean
}

export default function CameraForm({ initial, prefillRtsp, prefillName, onSave, onCancel, saving }: CameraFormProps) {
  const [form, setForm] = useState<CameraFormData>(() => {
    const base = emptyForm(initial)
    if (prefillRtsp) base.rtsp_url = prefillRtsp
    if (prefillName) base.name = prefillName
    return base
  })
  // editing mode when `initial` is provided
  const [detecting, setDetecting] = useState(false)
  const [detectMsg, setDetectMsg] = useState<{ text: string; ok: boolean } | null>(null)
  const [detectingLive, setDetectingLive] = useState(false)
  const [liveDetectMsg, setLiveDetectMsg] = useState<{ text: string; ok: boolean } | null>(null)
  const [liveRecommended, setLiveRecommended] = useState<string>('')

  const set = (field: keyof CameraFormData, value: string | boolean | number) =>
    setForm(prev => ({ ...prev, [field]: value }))

  const handleDetectSubstream = async () => {
    const main = form.rtsp_url.trim()
    if (!main) return
    setDetecting(true)
    setDetectMsg(null)
    try {
      const res = await fetch('/api/settings/cameras/detect-substream', {
        method: 'POST',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ rtsp_url: main, id: initial?.id }),
      })
      if (!res.ok) throw new Error('request failed')
      const data = (await res.json()) as { motion_rtsp_url?: string; width?: number; height?: number }
      if (data.motion_rtsp_url) {
        setForm(prev => ({ ...prev, motion_rtsp_url: data.motion_rtsp_url! }))
        setDetectMsg({ text: `Substream detectado: ${data.width}×${data.height}`, ok: true })
      } else {
        setDetectMsg({ text: 'Nenhum substream encontrado — informe manualmente.', ok: false })
      }
    } catch {
      setDetectMsg({ text: 'Erro ao detectar — verifique a URL principal.', ok: false })
    } finally {
      setDetecting(false)
    }
  }

  const handleDetectStreams = async () => {
    const main = form.rtsp_url.trim()
    if (!main) return
    setDetectingLive(true)
    setLiveDetectMsg(null)
    try {
      const res = await fetch('/api/settings/cameras/detect-streams', {
        method: 'POST',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ rtsp_url: main, id: initial?.id }),
      })
      if (!res.ok) throw new Error('request failed')
      const data = (await res.json()) as { codec?: string; width?: number; height?: number; recommended?: string }
      if (data.codec) {
        setLiveRecommended(data.recommended ?? '')
        const rec = data.recommended === 'webrtc'
          ? 'WebRTC recomendado (baixa latência)'
          : 'HLS recomendado — WebRTC indisponível para este codec'
        setLiveDetectMsg({ text: `Codec detectado: ${data.codec.toUpperCase()} — ${rec}`, ok: data.recommended === 'webrtc' })
      } else {
        setLiveRecommended('')
        setLiveDetectMsg({ text: 'Não foi possível detectar o codec — verifique a URL principal.', ok: false })
      }
    } catch {
      setLiveDetectMsg({ text: 'Erro ao detectar — verifique a URL principal.', ok: false })
    } finally {
      setDetectingLive(false)
    }
  }

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
          <label className={labelClass}>Nome</label>
          <input
            value={form.name}
            onChange={e => set('name', e.target.value)}
            required
            className={inputClass}
            placeholder="Sala, Garagem, Entrada"
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
        <div className="sm:col-span-2">
          <label className={labelClass}>RTSP URL da detecção de movimento (substream)</label>
          <div className="flex gap-2">
            <input
              id="camera-motion-rtsp-url"
              value={form.motion_rtsp_url}
              onChange={e => set('motion_rtsp_url', e.target.value)}
              className={inputClass}
              placeholder="rtsp://usuario:senha@ip:554/stream (subtype=1)"
            />
            <Button
              id="camera-motion-rtsp-detect"
              type="button"
              size="sm"
              variant="outline"
              disabled={!form.rtsp_url.trim() || detecting}
              onClick={handleDetectSubstream}
              className="shrink-0"
            >
              {detecting ? 'Detectando...' : 'Detectar'}
            </Button>
          </div>
          {detectMsg && (
            <p className={`text-xs mt-0.5 ${detectMsg.ok ? 'text-green-500' : 'text-amber-500'}`}>{detectMsg.text}</p>
          )}
          <p className="text-xs text-gray-400 mt-0.5">Opcional. Vazio = usa o stream principal. "Detectar" tenta descobrir o substream a partir da URL principal (menor resolução) — reduz muito o custo de CPU da detecção; o snapshot do evento sai nessa resolução.</p>
        </div>
        <div>
          <label className={labelClass}>Duração do chunk</label>
          <input value={form.chunk_duration} onChange={e => set('chunk_duration', e.target.value)} className={inputClass} placeholder="5m" />
          <p className="text-xs text-gray-400 mt-0.5">ex: 30s, 5m, 1h</p>
        </div>
        <div>
          <label className={labelClass}>Intervalo de reconexão</label>
          <input value={form.reconnect_interval} onChange={e => set('reconnect_interval', e.target.value)} className={inputClass} placeholder="30s" />
          <p className="text-xs text-gray-400 mt-0.5">ex: 10s, 1m, 5m</p>
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
          <label className={labelClass}>Modo de vídeo HLS</label>
          <select value={form.hls_video_mode} onChange={e => set('hls_video_mode', e.target.value)} className={inputClass}>
            <option value="auto">Auto (detecta via ffprobe)</option>
            <option value="h264">H.264 (sempre transcodifica)</option>
            <option value="copy">Cópia (sem transcodificação)</option>
          </select>
        </div>
        <div className="sm:col-span-2">
          <label className={labelClass}>Transporte do ao-vivo</label>
          <div className="flex gap-2">
            <select
              id="camera-live-transport"
              value={form.live_transport}
              onChange={e => set('live_transport', e.target.value)}
              className={inputClass}
            >
              <option value="auto">Automático — WebRTC com fallback HLS{liveRecommended === 'webrtc' ? ' (recomendado)' : ''}</option>
              <option value="webrtc">WebRTC — baixa latência{liveRecommended === 'webrtc' ? ' (recomendado)' : ''}</option>
              <option value="hls">HLS — compatível{liveRecommended === 'hls' ? ' (recomendado)' : ''}</option>
            </select>
            <Button
              id="camera-live-transport-detect"
              type="button"
              size="sm"
              variant="outline"
              disabled={!form.rtsp_url.trim() || detectingLive}
              onClick={handleDetectStreams}
              className="shrink-0"
            >
              {detectingLive ? 'Detectando...' : 'Detectar'}
            </Button>
          </div>
          {liveDetectMsg && (
            <p className={`text-xs mt-0.5 ${liveDetectMsg.ok ? 'text-green-500' : 'text-amber-500'}`}>{liveDetectMsg.text}</p>
          )}
          {form.live_transport === 'webrtc' && liveRecommended === 'hls' && (
            <p className="text-xs text-amber-500 mt-0.5">Este stream não é H.264 — o WebRTC cairá para HLS automaticamente.</p>
          )}
          <p className="text-xs text-gray-400 mt-0.5">WebRTC entrega o ao-vivo com latência abaixo de 1s (exige H.264 no stream principal). "Detectar" verifica o codec para recomendar o transporte.</p>
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
          <label className={labelClass}>Duração do segmento (s)</label>
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
            <span className="text-xs text-gray-500">Usar padrão (2 s)</span>
          </label>
          <p className="text-xs text-gray-400 mt-0.5">Cada segmento de vídeo ao vivo tem essa duração. Valores menores reduzem a latência, mas aumentam o processamento.</p>
        </div>

        <div>
          <label className={labelClass}>Janela de reprodução (segmentos)</label>
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
            <span className="text-xs text-gray-500">Usar padrão (5 segmentos)</span>
          </label>
          <p className="text-xs text-gray-400 mt-0.5">Quantidade de segmentos mantidos na playlist ao vivo. A latência aproximada é duração × janela (padrão ≈ 10 s).</p>
        </div>

        <div>
          <label className={labelClass}>Retenção DVR (s)</label>
          <input
            type="number"
            min={0}
            step={60}
            value={form.hls_dvr_seconds}
            onChange={e => set('hls_dvr_seconds', e.target.value)}
            className={inputClass}
            placeholder="0"
          />
          <p className="text-xs text-gray-400 mt-0.5">Tempo máximo, em segundos, que o histórico ao vivo fica disponível para consulta. Permite buscar eventos recentes sem gravação em disco. <span className="text-gray-400">0 = desativado.</span></p>
        </div>
      </div>

      <div className="border-t border-gray-800 pt-3">
        <p className="text-xs font-medium text-gray-400 mb-3">Gravação</p>
        <label className="flex items-center gap-2 cursor-pointer">
          <input
            type="checkbox"
            id="recording_enabled"
            checked={form.recording_enabled}
            onChange={e => set('recording_enabled', e.target.checked)}
            className="accent-blue-500"
          />
          <span className="text-xs text-gray-400">Gravar em disco</span>
        </label>
        {!form.recording_enabled && (
          <p className="text-xs text-gray-400 mt-1">HLS e detecção de movimento continuam funcionando</p>
        )}
      </div>

      <div className="flex gap-2">
        <Button id="camera-form-save" type="submit" size="sm" disabled={saving}>
          {saving ? 'Salvando...' : 'Salvar'}
        </Button>
        <Button id="camera-form-cancel" type="button" size="sm" variant="outline" onClick={onCancel}>
          Cancelar
        </Button>
      </div>
    </form>
  )
}
