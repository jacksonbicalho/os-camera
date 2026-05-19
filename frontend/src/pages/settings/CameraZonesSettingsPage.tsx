import { useCallback, useEffect, useRef, useState } from 'react'
import type HlsType from 'hls.js'
import { Link, useParams } from 'react-router-dom'
import SettingsLayout from '../../components/SettingsLayout'
import ConfirmDialog from '../../components/ConfirmDialog'
import { authHeaders, getToken } from '../../auth'
import { useSettings } from '../../hooks/useSettings'
import { useEventSource } from '../../hooks/useEventSource'

interface Zone {
  x: number
  y: number
  w: number
  h: number
  rotation_deg?: number
  type?: 'exclude' | 'detect'
  label?: string
  threshold?: number
  cooldown_seconds?: number
  fps?: number
  scale?: number
  color?: string
}

const ZONE_COLORS = [
  '#ef4444', // vermelho
  '#f97316', // laranja
  '#eab308', // amarelo
  '#22c55e', // verde
  '#06b6d4', // ciano
  '#3b82f6', // azul
  '#8b5cf6', // violeta
  '#ec4899', // rosa
]

function pickZoneColor(existing: Zone[]): string {
  const used = new Set(existing.map(z => z.color).filter(Boolean))
  return ZONE_COLORS.find(c => !used.has(c)) ?? ZONE_COLORS[existing.length % ZONE_COLORS.length]
}

function parseHex(hex: string): { r: number; g: number; b: number } {
  const h = hex.replace('#', '')
  return {
    r: parseInt(h.slice(0, 2), 16) || 0,
    g: parseInt(h.slice(2, 4), 16) || 0,
    b: parseInt(h.slice(4, 6), 16) || 0,
  }
}

type ResizeHandle = 'nw' | 'ne' | 'se' | 'sw'

type Interaction =
  | { mode: 'drawing'; startX: number; startY: number; curX: number; curY: number }
  | { mode: 'moving'; idx: number; orig: Zone; startX: number; startY: number; curX: number; curY: number }
  | { mode: 'resizing'; idx: number; orig: Zone; handle: ResizeHandle; startX: number; startY: number; curX: number; curY: number }
  | { mode: 'rotating'; idx: number; orig: Zone; startX: number; startY: number; curX: number; curY: number }
  | null

const HANDLE_PX = 10
const DELETE_PX = 12
const ROTATE_HANDLE_PX = 12
const ROTATE_OFFSET_PX = 22
const SNAP = 0.025

function clamp(v: number, lo: number, hi: number) { return Math.max(lo, Math.min(hi, v)) }
function snapEdge(v: number) {
  if (v < SNAP) return 0
  if (v > 1 - SNAP) return 1
  return v
}
function toNorm(px: number, dim: number) { return clamp(px / dim, 0, 1) }

function relPos(e: React.MouseEvent<HTMLCanvasElement>): { x: number; y: number } {
  const c = e.currentTarget
  const r = c.getBoundingClientRect()
  return {
    x: clamp((e.clientX - r.left) * c.width / r.width, 0, c.width),
    y: clamp((e.clientY - r.top) * c.height / r.height, 0, c.height),
  }
}

function dragToZone(x0: number, y0: number, x1: number, y1: number, cw: number, ch: number): Zone {
  const lx = Math.min(x0, x1); const rx = Math.max(x0, x1)
  const ly = Math.min(y0, y1); const ry = Math.max(y0, y1)
  const nx = snapEdge(toNorm(lx, cw)); const nx1 = snapEdge(toNorm(rx, cw))
  const ny = snapEdge(toNorm(ly, ch)); const ny1 = snapEdge(toNorm(ry, ch))
  return { x: nx, y: ny, w: nx1 - nx, h: ny1 - ny, type: 'exclude' }
}

function applyResize(orig: Zone, handle: ResizeHandle, dx: number, dy: number, cw: number, ch: number): Zone {
  const ndx = dx / cw; const ndy = dy / ch
  let { x, y, w, h } = orig
  switch (handle) {
    case 'nw':
      x = clamp(orig.x + ndx, 0, orig.x + orig.w - 0.02)
      y = clamp(orig.y + ndy, 0, orig.y + orig.h - 0.02)
      w = orig.x + orig.w - x; h = orig.y + orig.h - y
      break
    case 'ne':
      y = clamp(orig.y + ndy, 0, orig.y + orig.h - 0.02)
      h = orig.y + orig.h - y
      w = clamp(orig.w + ndx, 0.02, 1 - orig.x)
      break
    case 'se':
      w = clamp(orig.w + ndx, 0.02, 1 - orig.x)
      h = clamp(orig.h + ndy, 0.02, 1 - orig.y)
      break
    case 'sw':
      x = clamp(orig.x + ndx, 0, orig.x + orig.w - 0.02)
      w = orig.x + orig.w - x
      h = clamp(orig.h + ndy, 0.02, 1 - orig.y)
      break
  }
  return { ...orig, x, y, w, h }
}

function applyMove(orig: Zone, dx: number, dy: number, cw: number, ch: number): Zone {
  const nx = clamp(orig.x + dx / cw, 0, 1 - orig.w)
  const ny = clamp(orig.y + dy / ch, 0, 1 - orig.h)
  return { ...orig, x: nx, y: ny }
}

function normalizeDeg(v: number): number {
  let d = v % 360
  if (d < 0) d += 360
  return d
}

function zoneCenterPx(z: Zone, cw: number, ch: number): { cx: number; cy: number } {
  return { cx: (z.x + z.w / 2) * cw, cy: (z.y + z.h / 2) * ch }
}

function rotateHandlePos(z: Zone, cw: number, ch: number): { hx: number; hy: number } {
  const { cx, cy } = zoneCenterPx(z, cw, ch)
  const a = ((z.rotation_deg ?? 0) * Math.PI) / 180
  const topDist = z.h * ch / 2
  const localY = -(topDist + ROTATE_OFFSET_PX)
  return {
    hx: cx - localY * Math.sin(a),
    hy: cy + localY * Math.cos(a),
  }
}

function detectRotateHandle(zones: Zone[], selectedIdx: number | null, px: number, py: number, cw: number, ch: number): number {
  if (selectedIdx === null || !zones[selectedIdx]) return -1
  const { hx, hy } = rotateHandlePos(zones[selectedIdx], cw, ch)
  if (Math.hypot(px - hx, py - hy) <= ROTATE_HANDLE_PX) return selectedIdx
  return -1
}

function detectHandle(zones: Zone[], px: number, py: number, cw: number, ch: number): { idx: number; handle: ResizeHandle } | null {
  for (let i = zones.length - 1; i >= 0; i--) {
    const z = zones[i]
    const x0 = z.x * cw; const y0 = z.y * ch
    const x1 = (z.x + z.w) * cw; const y1 = (z.y + z.h) * ch
    const corners: [ResizeHandle, number, number][] = [
      ['nw', x0, y0], ['ne', x1, y0], ['se', x1, y1], ['sw', x0, y1],
    ]
    for (const [handle, cx, cy] of corners) {
      if (Math.abs(px - cx) <= HANDLE_PX && Math.abs(py - cy) <= HANDLE_PX) {
        return { idx: i, handle }
      }
    }
  }
  return null
}

function detectDeleteButton(zones: Zone[], px: number, py: number, cw: number, ch: number): number {
  for (let i = zones.length - 1; i >= 0; i--) {
    const z = zones[i]
    const bx = (z.x + z.w) * cw - DELETE_PX
    const by = z.y * ch + DELETE_PX
    if (Math.abs(px - bx) <= DELETE_PX && Math.abs(py - by) <= DELETE_PX) {
      return i
    }
  }
  return -1
}

function hitTest(zones: Zone[], px: number, py: number, cw: number, ch: number): number {
  for (let i = zones.length - 1; i >= 0; i--) {
    const z = zones[i]
    if (px >= z.x * cw && px <= (z.x + z.w) * cw &&
        py >= z.y * ch && py <= (z.y + z.h) * ch) {
      return i
    }
  }
  return -1
}

const HANDLE_CURSORS: Record<ResizeHandle, string> = {
  nw: 'nw-resize', ne: 'ne-resize', se: 'se-resize', sw: 'sw-resize',
}

function zoneColor(z: Zone, selected: boolean): { fill: string; stroke: string; handle: string } {
  const hex = z.color ?? (z.type === 'detect' ? '#f97316' : '#ef4444')
  const { r, g, b } = parseHex(hex)
  const fillOpacity = selected ? 0.30 : 0.18
  const strokeOpacity = selected ? 1.0 : 0.85
  return {
    fill: `rgba(${r},${g},${b},${fillOpacity})`,
    stroke: `rgba(${r},${g},${b},${strokeOpacity})`,
    handle: `rgba(${r},${g},${b},0.95)`,
  }
}

function drawDeleteButton(ctx: CanvasRenderingContext2D, cx: number, cy: number, strokeColor: string) {
  const r = 9
  ctx.beginPath()
  ctx.arc(cx, cy, r, 0, Math.PI * 2)
  ctx.fillStyle = 'rgba(30,30,30,0.85)'
  ctx.fill()
  ctx.strokeStyle = strokeColor
  ctx.lineWidth = 1.2
  ctx.stroke()
  const d = 4
  ctx.beginPath()
  ctx.moveTo(cx - d, cy - d); ctx.lineTo(cx + d, cy + d)
  ctx.moveTo(cx + d, cy - d); ctx.lineTo(cx - d, cy + d)
  ctx.strokeStyle = strokeColor
  ctx.lineWidth = 1.8
  ctx.stroke()
}

function paintCanvas(
  ctx: CanvasRenderingContext2D,
  video: HTMLVideoElement | null,
  zones: Zone[],
  cw: number,
  ch: number,
  ia: Interaction,
  selectedIdx: number | null,
) {
  ctx.clearRect(0, 0, cw, ch)
  if (video && video.readyState >= 2) {
    ctx.drawImage(video, 0, 0, cw, ch)
  } else {
    ctx.fillStyle = '#1f2937'
    ctx.fillRect(0, 0, cw, ch)
    ctx.fillStyle = '#6b7280'
    ctx.font = '14px sans-serif'
    ctx.textAlign = 'center'
    ctx.fillText('Aguardando transmissão...', cw / 2, ch / 2)
  }

  const preview: Zone[] = zones.map((z, i) => {
    if (ia?.mode === 'moving' && ia.idx === i)
      return applyMove(ia.orig, ia.curX - ia.startX, ia.curY - ia.startY, cw, ch)
    if (ia?.mode === 'resizing' && ia.idx === i)
      return applyResize(ia.orig, ia.handle, ia.curX - ia.startX, ia.curY - ia.startY, cw, ch)
    if (ia?.mode === 'rotating' && ia.idx === i) {
      const { cx, cy } = zoneCenterPx(ia.orig, cw, ch)
      const deg = normalizeDeg(Math.atan2(ia.curY - cy, ia.curX - cx) * 180 / Math.PI + 90)
      return { ...ia.orig, rotation_deg: deg }
    }
    return z
  })
  if (ia?.mode === 'drawing') {
    preview.push(dragToZone(ia.startX, ia.startY, ia.curX, ia.curY, cw, ch))
  }

  for (let i = 0; i < preview.length; i++) {
    const z = preview[i]
    const px = z.x * cw; const py = z.y * ch; const pw = z.w * cw; const ph = z.h * ch
    const isDrawing = ia?.mode === 'drawing' && i === preview.length - 1
    const isSelected = !isDrawing && selectedIdx === i
    const colors = zoneColor(z, isSelected)

    const cx = px + pw / 2
    const cy = py + ph / 2
    const a = ((z.rotation_deg ?? 0) * Math.PI) / 180
    ctx.save()
    ctx.translate(cx, cy)
    ctx.rotate(a)
    ctx.fillStyle = colors.fill
    ctx.fillRect(-pw / 2, -ph / 2, pw, ph)
    ctx.strokeStyle = colors.stroke
    ctx.lineWidth = isSelected ? 2 : 1.5
    ctx.strokeRect(-pw / 2, -ph / 2, pw, ph)
    ctx.restore()

    if (!isDrawing) {
      const corners: [number, number][] = [[px, py], [px + pw, py], [px + pw, py + ph], [px, py + ph]]
      for (const [hx, hy] of corners) {
        ctx.fillStyle = colors.handle
        ctx.fillRect(hx - 5, hy - 5, 10, 10)
      }
      if (isSelected) {
        const { cx, cy } = zoneCenterPx(z, cw, ch)
        const { hx, hy } = rotateHandlePos(z, cw, ch)
        ctx.beginPath()
        ctx.moveTo(cx, cy - ph / 2)
        ctx.lineTo(hx, hy)
        ctx.strokeStyle = colors.handle
        ctx.lineWidth = 1.2
        ctx.stroke()
        ctx.beginPath()
        ctx.arc(hx, hy, 7, 0, Math.PI * 2)
        ctx.fillStyle = colors.handle
        ctx.fill()
      }
      drawDeleteButton(ctx, px + pw - DELETE_PX, py + DELETE_PX, colors.stroke)

      // label / type badge
      const displayLabel = z.label || (z.type === 'detect' ? 'detect' : null)
      if (displayLabel) {
        ctx.font = '11px sans-serif'
        ctx.textAlign = 'left'
        ctx.fillStyle = 'rgba(0,0,0,0.6)'
        ctx.fillRect(px + 4, py + 4, ctx.measureText(displayLabel).width + 8, 16)
        ctx.fillStyle = colors.stroke
        ctx.fillText(displayLabel, px + 8, py + 15)
      }
    }
  }
}

export default function CameraZonesSettingsPage() {
  const { id } = useParams<{ id: string }>()
  const { settings } = useSettings(`/settings/cameras/${id}/zones`)
  const cam = settings?.cameras.find(c => c.id === id)

  const capW = cam
    ? ((cam.motion?.capture_width ?? 0) > 0 ? cam.motion!.capture_width! : Math.max(1, Math.round((cam.width ?? 0) / 4)))
    : 0
  const capH = cam
    ? ((cam.motion?.capture_height ?? 0) > 0 ? cam.motion!.capture_height! : Math.max(1, Math.round((cam.height ?? 0) / 4)))
    : 0

  const [zones, setZones] = useState<Zone[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [toast, setToast] = useState<{ msg: string; ok: boolean } | null>(null)
  const [confirmClear, setConfirmClear] = useState(false)
  const [interaction, setInteraction] = useState<Interaction>(null)
  const [cursorStyle, setCursorStyle] = useState('crosshair')
  const [selectedIdx, setSelectedIdx] = useState<number | null>(null)
  const [regionScore, setRegionScore] = useState<number | null>(null)
  const [peakScore, setPeakScore] = useState<number | null>(null)

  const canvasRef = useRef<HTMLCanvasElement>(null)
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const hlsRef = useRef<HlsType | null>(null)
  const rafRef = useRef<number>(0)
  const zonesRef = useRef(zones)
  const interactionRef = useRef(interaction)
  const selectedIdxRef = useRef(selectedIdx)
  useEffect(() => {
    zonesRef.current = zones
    interactionRef.current = interaction
    selectedIdxRef.current = selectedIdx
  })

  // SSE URL for live region score — only when a zone is selected
  const sseURL = (() => {
    if (selectedIdx === null || !zones[selectedIdx] || !id) return null
    const z = zones[selectedIdx]
    return `/api/cameras/${id}/motion/region-score?x=${z.x}&y=${z.y}&w=${z.w}&h=${z.h}`
  })()

  const handleScoreMessage = useCallback((data: string) => {
    try {
      const p = JSON.parse(data)
      if (typeof p.score === 'number') {
        setRegionScore(p.score)
        setPeakScore(prev => prev === null ? p.score : Math.max(prev, p.score))
      }
    } catch {
      // ignore JSON parse errors
    }
  }, [])

  useEventSource(sseURL, handleScoreMessage)

  useEffect(() => {
    if (!id) return
    let active = true
    fetch(`/api/cameras/${id}/motion/zones`, { headers: authHeaders() })
      .then(r => r.json())
      .then((data: Zone[]) => { if (!active) return; setZones(data ?? []); setLoading(false) })
      .catch(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [id])

  // HLS live stream setup
  useEffect(() => {
    if (!id) return
    const video = document.createElement('video')
    video.muted = true
    video.playsInline = true
    videoRef.current = video

    import('hls.js').then(({ default: Hls }) => {
      const src = `/stream/${id}/index.m3u8`
      if (!Hls.isSupported()) {
        video.src = `${src}?token=${encodeURIComponent(getToken() ?? '')}`
        video.play().catch(() => {})
        return
      }
      const hls = new Hls({
        manifestLoadingMaxRetry: 20,
        manifestLoadingRetryDelay: 2000,
        xhrSetup(xhr) {
          xhr.setRequestHeader('Authorization', `Bearer ${getToken()}`)
        },
      })
      hls.loadSource(src)
      hls.attachMedia(video)
      hls.on(Hls.Events.MANIFEST_PARSED, () => video.play().catch(() => {}))
      hlsRef.current = hls
    })

    return () => {
      hlsRef.current?.destroy()
      hlsRef.current = null
      video.src = ''
      videoRef.current = null
    }
  }, [id])

  // RAF loop — repinta o canvas a cada frame com o vídeo ao vivo e as zonas
  useEffect(() => {
    function loop() {
      const canvas = canvasRef.current
      const ctx = canvas?.getContext('2d')
      if (canvas && ctx) {
        paintCanvas(ctx, videoRef.current, zonesRef.current, canvas.width, canvas.height, interactionRef.current, selectedIdxRef.current)
      }
      rafRef.current = requestAnimationFrame(loop)
    }
    rafRef.current = requestAnimationFrame(loop)
    return () => cancelAnimationFrame(rafRef.current)
  }, [])


  function updateCursor(px: number, py: number, currentZones: Zone[]) {
    const canvas = canvasRef.current
    if (!canvas) return
    const cw = canvas.width; const ch = canvas.height
    const del = detectDeleteButton(currentZones, px, py, cw, ch)
    if (del >= 0) { setCursorStyle('pointer'); return }
    const rot = detectRotateHandle(currentZones, selectedIdxRef.current, px, py, cw, ch)
    if (rot >= 0) { setCursorStyle('alias'); return }
    const h = detectHandle(currentZones, px, py, cw, ch)
    if (h) { setCursorStyle(HANDLE_CURSORS[h.handle]); return }
    const hit = hitTest(currentZones, px, py, cw, ch)
    setCursorStyle(hit >= 0 ? 'grab' : 'crosshair')
  }

  function updateSelectedZone(patch: Partial<Zone>) {
    if (selectedIdx === null) return
    setZones(prev => prev.map((z, i) => i === selectedIdx ? { ...z, ...patch } : z))
  }

  function handleMouseDown(e: React.MouseEvent<HTMLCanvasElement>) {
    const { x, y } = relPos(e)
    const canvas = canvasRef.current
    if (!canvas) return
    const cw = canvas.width; const ch = canvas.height

    const delIdx = detectDeleteButton(zones, x, y, cw, ch)
    if (delIdx >= 0) {
      setSelectedIdx(prev => {
        if (prev === delIdx) { setRegionScore(null); setPeakScore(null); return null }
        if (prev !== null && delIdx < prev) return prev - 1
        return prev
      })
      setZones(prev => prev.filter((_, i) => i !== delIdx))
      return
    }
    const rotIdx = detectRotateHandle(zones, selectedIdx, x, y, cw, ch)
    if (rotIdx >= 0) {
      if (rotIdx !== selectedIdx) { setRegionScore(null); setPeakScore(null) }
      setSelectedIdx(rotIdx)
      setInteraction({ mode: 'rotating', idx: rotIdx, orig: zones[rotIdx], startX: x, startY: y, curX: x, curY: y })
      setCursorStyle('alias')
      return
    }
    const h = detectHandle(zones, x, y, cw, ch)
    if (h) {
      if (h.idx !== selectedIdx) { setRegionScore(null); setPeakScore(null) }
      setSelectedIdx(h.idx)
      setInteraction({ mode: 'resizing', idx: h.idx, orig: zones[h.idx], handle: h.handle, startX: x, startY: y, curX: x, curY: y })
      setCursorStyle(HANDLE_CURSORS[h.handle])
      return
    }
    const idx = hitTest(zones, x, y, cw, ch)
    if (idx >= 0) {
      if (idx !== selectedIdx) { setRegionScore(null); setPeakScore(null) }
      setSelectedIdx(idx)
      setInteraction({ mode: 'moving', idx, orig: zones[idx], startX: x, startY: y, curX: x, curY: y })
      setCursorStyle('grabbing')
      return
    }
    setSelectedIdx(null)
    setRegionScore(null)
    setPeakScore(null)
    setInteraction({ mode: 'drawing', startX: x, startY: y, curX: x, curY: y })
  }

  function handleMouseMove(e: React.MouseEvent<HTMLCanvasElement>) {
    const { x, y } = relPos(e)
    if (!interaction) { updateCursor(x, y, zones); return }
    setInteraction(ia => ia ? { ...ia, curX: x, curY: y } : null)
  }

  function handleMouseUp(e: React.MouseEvent<HTMLCanvasElement>) {
    e.preventDefault()
    const canvas = canvasRef.current
    if (!canvas || !interaction) { setInteraction(null); return }
    const cw = canvas.width; const ch = canvas.height

    if (interaction.mode === 'drawing') {
      const z = dragToZone(interaction.startX, interaction.startY, interaction.curX, interaction.curY, cw, ch)
      if (z.w > 0.01 && z.h > 0.01) {
        const newIdx = zones.length
        setRegionScore(null)
        setPeakScore(null)
        setZones(prev => {
          const color = pickZoneColor(prev)
          return [...prev, { ...z, color }]
        })
        setSelectedIdx(newIdx)
      }
    } else if (interaction.mode === 'moving') {
      const moved = applyMove(interaction.orig, interaction.curX - interaction.startX, interaction.curY - interaction.startY, cw, ch)
      setZones(prev => prev.map((z, i) => i === interaction.idx ? moved : z))
    } else if (interaction.mode === 'resizing') {
      const resized = applyResize(interaction.orig, interaction.handle, interaction.curX - interaction.startX, interaction.curY - interaction.startY, cw, ch)
      setZones(prev => prev.map((z, i) => i === interaction.idx ? resized : z))
    } else if (interaction.mode === 'rotating') {
      const { cx, cy } = zoneCenterPx(interaction.orig, cw, ch)
      const deg = normalizeDeg(Math.atan2(interaction.curY - cy, interaction.curX - cx) * 180 / Math.PI + 90)
      setZones(prev => prev.map((z, i) => i === interaction.idx ? { ...z, rotation_deg: deg } : z))
    }
    setInteraction(null)
    const { x, y } = relPos(e)
    updateCursor(x, y, zones)
  }

  async function save() {
    setSaving(true)
    try {
      const r = await fetch(`/api/cameras/${id}/motion/zones`, {
        method: 'PUT',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify(zones),
      })
      setToast({ msg: r.ok ? 'Zonas salvas.' : 'Erro ao salvar.', ok: r.ok })
    } catch {
      setToast({ msg: 'Erro ao salvar.', ok: false })
    } finally {
      setSaving(false)
      setTimeout(() => setToast(null), 3000)
    }
  }

  const selectedZone = selectedIdx !== null ? zones[selectedIdx] : null

  return (
    <SettingsLayout>
      <nav className="flex items-center gap-1.5 text-xs text-gray-500 mb-5">
        <Link to="/settings/cameras" className="hover:text-gray-300 transition-colors">Câmeras</Link>
        <span>/</span>
        <Link to={`/settings/cameras/${id}`} className="hover:text-gray-300 transition-colors">{id}</Link>
        <span>/</span>
        <Link to={`/settings/cameras/${id}/motion`} className="hover:text-gray-300 transition-colors">Detecção de movimento</Link>
        <span>/</span>
        <span className="text-gray-300">Zonas</span>
      </nav>

      <h2 className="text-lg font-semibold text-gray-200 mb-2">{id} — zonas</h2>
      <p className="text-xs text-gray-500 mb-5">
        Arraste em área vazia para criar uma zona. Clique numa zona para selecioná-la e configurá-la. Arraste os cantos para redimensionar. Clique no × para excluir.
      </p>

      {!settings ? (
        <p className="text-gray-500 text-sm">Carregando...</p>
      ) : !cam ? (
        <p className="text-gray-500 text-sm">Câmera não encontrada.</p>
      ) : loading ? (
        <p className="text-gray-500 text-sm">Carregando zonas...</p>
      ) : (
        <div className="flex flex-col gap-4">
          {/* Canvas — fundo: transmissão ao vivo via HLS */}
          <div
            className="relative bg-gray-900 border border-gray-800 rounded-lg overflow-hidden"
            style={{ aspectRatio: '16/9' }}
          >
            <canvas
              ref={canvasRef}
              width={960}
              height={540}
              className="w-full h-full select-none"
              style={{ cursor: cursorStyle }}
              onMouseDown={handleMouseDown}
              onMouseMove={handleMouseMove}
              onMouseUp={handleMouseUp}
              onMouseLeave={() => {
                if (interaction) handleMouseUp({ preventDefault: () => {} } as React.MouseEvent<HTMLCanvasElement>)
                setCursorStyle('crosshair')
              }}
            />
          </div>

          {/* Zone settings panel */}
          {selectedZone && (
            <div className="bg-gray-900 border border-gray-800 rounded-lg p-4 flex flex-col gap-4">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-medium text-gray-200 flex items-center gap-2">
                  <span
                    className="inline-block w-3 h-3 rounded-sm flex-shrink-0"
                    style={{ backgroundColor: selectedZone.color ?? (selectedZone.type === 'detect' ? '#f97316' : '#ef4444') }}
                  />
                  Zona {selectedIdx! + 1}
                  {selectedZone.type === 'detect' && (
                    <span className="text-xs text-gray-400 font-normal">detecção independente</span>
                  )}
                </h3>
                {sseURL && (
                  <div className="flex items-center gap-3 flex-wrap">
                    <div className="flex items-center gap-1.5">
                      <span className="text-xs text-gray-500">Ao vivo:</span>
                      <span className="text-sm font-mono text-yellow-400 min-w-[6ch]">
                        {regionScore !== null ? regionScore.toFixed(4) : '—'}
                      </span>
                    </div>
                    <div className="flex items-center gap-1.5">
                      <span className="text-xs text-gray-500">Pico:</span>
                      <span className="text-sm font-mono text-orange-400 min-w-[6ch]">
                        {peakScore !== null ? peakScore.toFixed(4) : '—'}
                      </span>
                    </div>
                  </div>
                )}
              </div>

              <div className="flex gap-6 flex-wrap">
                {/* Label */}
                <div className="flex flex-col gap-1 min-w-40">
                  <label className="text-xs text-gray-400">Nome (opcional)</label>
                  <input
                    type="text"
                    value={selectedZone.label ?? ''}
                    onChange={e => updateSelectedZone({ label: e.target.value || undefined })}
                    placeholder="ex: entrada"
                    className="bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-sm text-gray-200 focus:outline-none focus:border-gray-600 w-full"
                  />
                </div>

                <div className="flex flex-col gap-1 min-w-32">
                  <label className="text-xs text-gray-400">Rotação (graus)</label>
                  <input
                    type="number"
                    min={0}
                    max={360}
                    step={0.1}
                    value={Math.round(((selectedZone.rotation_deg ?? 0) + Number.EPSILON) * 10) / 10}
                    onChange={e => updateSelectedZone({ rotation_deg: normalizeDeg(parseFloat(e.target.value) || 0) })}
                    className="bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-sm text-gray-200 focus:outline-none focus:border-gray-600 w-28"
                  />
                </div>

                {/* Type */}
                <div className="flex flex-col gap-2">
                  <span className="text-xs text-gray-400">Tipo</span>
                  <div className="flex gap-4">
                    {(['exclude', 'detect'] as const).map(t => (
                      <label key={t} className="flex items-center gap-1.5 text-sm text-gray-300 cursor-pointer">
                        <input
                          type="radio"
                          value={t}
                          checked={(selectedZone.type ?? 'exclude') === t}
                          onChange={() => updateSelectedZone({ type: t })}
                          className="accent-blue-500"
                        />
                        {t === 'exclude' ? 'Exclusão' : 'Detecção'}
                      </label>
                    ))}
                  </div>
                  <p className="text-xs text-gray-600 max-w-xs">
                    {(selectedZone.type ?? 'exclude') === 'exclude'
                      ? 'Ignora movimento nesta região no diff global.'
                      : 'Detecta movimento nesta região de forma independente, com limiar e cooldown próprios.'}
                  </p>
                </div>

                {/* Detect-only fields */}
                {(selectedZone.type ?? 'exclude') === 'detect' && (
                  <>
                    <div className="flex flex-col gap-1">
                      <label className="text-xs text-gray-400">Limiar (0 = câmera)</label>
                      <input
                        type="number"
                        min={0} max={1} step={0.001}
                        value={selectedZone.threshold ?? 0}
                        onChange={e => updateSelectedZone({ threshold: parseFloat(e.target.value) || 0 })}
                        className="bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-sm text-gray-200 focus:outline-none focus:border-gray-600 w-28"
                      />
                    </div>
                    <div className="flex flex-col gap-1">
                      <label className="text-xs text-gray-400">Cooldown (s, 0 = câmera)</label>
                      <input
                        type="number"
                        min={0} step={1}
                        value={selectedZone.cooldown_seconds ?? 0}
                        onChange={e => updateSelectedZone({ cooldown_seconds: parseInt(e.target.value) || 0 })}
                        className="bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-sm text-gray-200 focus:outline-none focus:border-gray-600 w-28"
                      />
                    </div>
                    <div className="flex flex-col gap-1">
                      <label className="text-xs text-gray-400">FPS de amostragem (0 = câmera)</label>
                      <input
                        type="number"
                        min={0} step={1}
                        value={selectedZone.fps ?? 0}
                        onChange={e => updateSelectedZone({ fps: parseInt(e.target.value) || 0 })}
                        className="bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-sm text-gray-200 focus:outline-none focus:border-gray-600 w-28"
                      />
                    </div>
                    <div className="flex flex-col gap-1.5">
                      <label className="text-xs text-gray-400">Escala de análise</label>
                      <div className="flex items-center gap-3">
                        <input
                          type="range"
                          min={10} max={100} step={5}
                          value={Math.round((selectedZone.scale || 1) * 100)}
                          onChange={e => updateSelectedZone({ scale: parseInt(e.target.value) / 100 })}
                          className="w-32 accent-blue-500"
                        />
                        <span className="text-xs text-gray-300 font-mono w-10 text-right">
                          {Math.round((selectedZone.scale || 1) * 100)}%
                        </span>
                      </div>
                      {capW > 0 && capH > 0 && (() => {
                        const zW = Math.max(1, Math.round(selectedZone.w * capW))
                        const zH = Math.max(1, Math.round(selectedZone.h * capH))
                        const sc = selectedZone.scale || 1
                        const sW = Math.max(1, Math.round(zW * sc))
                        const sH = Math.max(1, Math.round(zH * sc))
                        return (
                          <p className="text-xs text-gray-600">
                            {sc < 1 ? `${zW} × ${zH} px → ${sW} × ${sH} px` : `${zW} × ${zH} px`}
                          </p>
                        )
                      })()}
                    </div>
                  </>
                )}
              </div>
            </div>
          )}

          {toast && (
            <p className={`text-sm ${toast.ok ? 'text-green-400' : 'text-red-400'}`}>{toast.msg}</p>
          )}

          <div className="flex gap-3 items-center">
            <button
              onClick={save}
              disabled={saving}
              className="px-4 py-2 text-sm bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg transition-colors"
            >
              {saving ? 'Salvando...' : 'Salvar zonas'}
            </button>
            {zones.length > 0 && (
              <button
                onClick={() => setConfirmClear(true)}
                className="px-4 py-2 text-sm bg-gray-800 hover:bg-gray-700 text-gray-300 rounded-lg transition-colors"
              >
                Limpar todas
              </button>
            )}
            {zones.length > 0 && (
              <span className="text-xs text-gray-600">{zones.length} zona{zones.length !== 1 ? 's' : ''}</span>
            )}
          </div>
        </div>
      )}

      <ConfirmDialog
        open={confirmClear}
        title="Limpar todas as zonas"
        message="Todas as zonas serão removidas. Esta ação não pode ser desfeita."
        confirmLabel="Limpar"
        danger
        onConfirm={() => { setZones([]); setSelectedIdx(null); setConfirmClear(false) }}
        onCancel={() => setConfirmClear(false)}
      />
    </SettingsLayout>
  )
}
