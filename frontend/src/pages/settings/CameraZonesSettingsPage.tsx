import { useCallback, useEffect, useRef, useState } from 'react'
import type HlsType from 'hls.js'
import { useParams } from 'react-router-dom'
import SettingsLayout from '../../components/SettingsLayout'
import CameraSettingsTabs from '../../components/CameraSettingsTabs'
import ConfirmDialog from '../../components/ConfirmDialog'
import { authHeaders, getRole, getToken } from '../../auth'
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
  '#ef4444', '#f97316', '#eab308', '#22c55e',
  '#06b6d4', '#3b82f6', '#8b5cf6', '#ec4899',
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

// ── geometry ──────────────────────────────────────────────────────────────────

function clamp(v: number, lo: number, hi: number) { return Math.max(lo, Math.min(hi, v)) }
const SNAP = 0.025
function snapEdge(v: number) { return v < SNAP ? 0 : v > 1 - SNAP ? 1 : v }
function deg2rad(d: number) { return (((d ?? 0) % 360 + 360) % 360) * Math.PI / 180 }

// Rotate local vector (lx, ly) by angle a and offset by world center (cx, cy)
function toWorld(cx: number, cy: number, lx: number, ly: number, a: number): [number, number] {
  return [
    cx + lx * Math.cos(a) - ly * Math.sin(a),
    cy + lx * Math.sin(a) + ly * Math.cos(a),
  ]
}

// Map world point (wx, wy) into local frame of zone (center cx/cy, angle a)
function toLocal(cx: number, cy: number, wx: number, wy: number, a: number): [number, number] {
  const dx = wx - cx, dy = wy - cy
  return [dx * Math.cos(a) + dy * Math.sin(a), -dx * Math.sin(a) + dy * Math.cos(a)]
}

function zoneCenter(z: Zone, cw: number, ch: number): [number, number] {
  return [(z.x + z.w / 2) * cw, (z.y + z.h / 2) * ch]
}

// Corners: 0=TL 1=TR 2=BR 3=BL (local signs)
const CORNER_SIGNS: [number, number][] = [[-1, -1], [1, -1], [1, 1], [-1, 1]]
const OPP_CORNER = [2, 3, 0, 1]
const CORNER_CURSORS = ['nw-resize', 'ne-resize', 'se-resize', 'sw-resize']
const HANDLE_R = 6   // resize handle hit radius
const ROT_R = 8      // rotation handle hit radius
const ROT_OFFSET = 26 // px above top edge

function cornerWorld(z: Zone, c: number, cw: number, ch: number): [number, number] {
  const [cx, cy] = zoneCenter(z, cw, ch)
  const hw = (z.w * cw) / 2, hh = (z.h * ch) / 2
  const a = deg2rad(z.rotation_deg ?? 0)
  const [sx, sy] = CORNER_SIGNS[c]
  return toWorld(cx, cy, sx * hw, sy * hh, a)
}

// Rotation handle: above the top edge, along the zone's up-axis
function rotHandleWorld(z: Zone, cw: number, ch: number): [number, number] {
  const [cx, cy] = zoneCenter(z, cw, ch)
  const hh = (z.h * ch) / 2
  const a = deg2rad(z.rotation_deg ?? 0)
  return toWorld(cx, cy, 0, -(hh + ROT_OFFSET), a)
}

// Delete button: just outside TR corner, shifted in local (+x, -y) direction
const DEL_R = 9
const DEL_OFFSET = DEL_R + 2

function delBtnWorld(z: Zone, cw: number, ch: number): [number, number] {
  const [cx, cy] = zoneCenter(z, cw, ch)
  const hw = (z.w * cw) / 2, hh = (z.h * ch) / 2
  const a = deg2rad(z.rotation_deg ?? 0)
  const off = DEL_OFFSET / Math.SQRT2
  return toWorld(cx, cy, hw + off, -hh - off, a)
}

function hitZone(z: Zone, px: number, py: number, cw: number, ch: number): boolean {
  const [cx, cy] = zoneCenter(z, cw, ch)
  const hw = (z.w * cw) / 2, hh = (z.h * ch) / 2
  const a = deg2rad(z.rotation_deg ?? 0)
  const [lx, ly] = toLocal(cx, cy, px, py, a)
  return Math.abs(lx) < hw && Math.abs(ly) < hh
}

function hitCorner(z: Zone, px: number, py: number, cw: number, ch: number): number {
  for (let c = 0; c < 4; c++) {
    const [wx, wy] = cornerWorld(z, c, cw, ch)
    if (Math.hypot(px - wx, py - wy) <= HANDLE_R + 3) return c
  }
  return -1
}

// Clamp handle to canvas so it's always reachable even when zone is near an edge
function rotHandleVisible(z: Zone, cw: number, ch: number): [number, number] {
  const [hx, hy] = rotHandleWorld(z, cw, ch)
  return [clamp(hx, ROT_R + 4, cw - ROT_R - 4), clamp(hy, ROT_R + 4, ch - ROT_R - 4)]
}

function hitRotHandle(z: Zone, px: number, py: number, cw: number, ch: number): boolean {
  const [hx, hy] = rotHandleVisible(z, cw, ch)
  return Math.hypot(px - hx, py - hy) <= ROT_R + 3
}

function hitDelBtn(z: Zone, px: number, py: number, cw: number, ch: number): boolean {
  const [bx, by] = delBtnWorld(z, cw, ch)
  return Math.hypot(px - bx, py - by) <= DEL_R + 3
}

// Move zone by (dx, dy) canvas pixels, keeping center on canvas
function moveZone(z: Zone, dx: number, dy: number, cw: number, ch: number): Zone {
  const [cx, cy] = zoneCenter(z, cw, ch)
  const hw = (z.w * cw) / 2, hh = (z.h * ch) / 2
  const ncx = clamp(cx + dx, hw, cw - hw)
  const ncy = clamp(cy + dy, hh, ch - hh)
  return { ...z, x: (ncx - hw) / cw, y: (ncy - hh) / ch }
}

// Resize by dragging corner c to (mx, my). Opposite corner stays fixed.
function resizeZone(z: Zone, c: number, mx: number, my: number, cw: number, ch: number): Zone {
  const [fx, fy] = cornerWorld(z, OPP_CORNER[c], cw, ch)
  const ncx = (fx + mx) / 2, ncy = (fy + my) / 2
  const a = deg2rad(z.rotation_deg ?? 0)
  const [lx, ly] = toLocal(ncx, ncy, fx, fy, a)
  const hw = Math.max(0.015 * cw, Math.abs(lx))
  const hh = Math.max(0.015 * ch, Math.abs(ly))
  const nx = clamp((ncx - hw) / cw, 0, 1)
  const ny = clamp((ncy - hh) / ch, 0, 1)
  const nw = clamp((2 * hw) / cw, 0.02, 1 - nx)
  const nh = clamp((2 * hh) / ch, 0.02, 1 - ny)
  return { ...z, x: nx, y: ny, w: nw, h: nh }
}

// Rotate zone so its up-axis points towards (mx, my)
function rotateZone(z: Zone, mx: number, my: number, cw: number, ch: number): Zone {
  const [cx, cy] = zoneCenter(z, cw, ch)
  let deg = Math.atan2(mx - cx, -(my - cy)) * 180 / Math.PI
  if (deg < 0) deg += 360
  return { ...z, rotation_deg: deg }
}

// ── canvas painting ───────────────────────────────────────────────────────────

function drawDeleteButton(ctx: CanvasRenderingContext2D, bx: number, by: number, strokeColor: string) {
  ctx.beginPath()
  ctx.arc(bx, by, DEL_R, 0, Math.PI * 2)
  ctx.fillStyle = 'rgba(30,30,30,0.85)'
  ctx.fill()
  ctx.strokeStyle = strokeColor
  ctx.lineWidth = 1.2
  ctx.stroke()
  const d = 3.5
  ctx.beginPath()
  ctx.moveTo(bx - d, by - d); ctx.lineTo(bx + d, by + d)
  ctx.moveTo(bx + d, by - d); ctx.lineTo(bx - d, by + d)
  ctx.strokeStyle = strokeColor
  ctx.lineWidth = 1.8
  ctx.stroke()
}

type Interaction =
  | null
  | { mode: 'drawing'; x0: number; y0: number; x1: number; y1: number }
  | { mode: 'moving'; idx: number; orig: Zone; x0: number; y0: number; x1: number; y1: number }
  | { mode: 'resizing'; idx: number; corner: number; x1: number; y1: number }
  | { mode: 'rotating'; idx: number; x1: number; y1: number }

function paintCanvas(
  ctx: CanvasRenderingContext2D,
  video: HTMLVideoElement | null,
  zones: Zone[],
  cw: number,
  ch: number,
  ia: Interaction,
  selIdx: number | null,
  readonly: boolean,
) {
  ctx.clearRect(0, 0, cw, ch)

  if (video && video.readyState >= 2) {
    ctx.drawImage(video, 0, 0, cw, ch)
  } else {
    ctx.fillStyle = '#111827'
    ctx.fillRect(0, 0, cw, ch)
    ctx.fillStyle = '#6b7280'
    ctx.font = '14px sans-serif'
    ctx.textAlign = 'center'
    ctx.fillText('Aguardando transmissão...', cw / 2, ch / 2)
    ctx.textAlign = 'left'
  }

  // Build display list (active interaction applied)
  const display: Zone[] = zones.map((z, i) => {
    if (!ia) return z
    if (ia.mode === 'moving' && ia.idx === i) return moveZone(ia.orig, ia.x1 - ia.x0, ia.y1 - ia.y0, cw, ch)
    if (ia.mode === 'resizing' && ia.idx === i) return resizeZone(z, ia.corner, ia.x1, ia.y1, cw, ch)
    if (ia.mode === 'rotating' && ia.idx === i) return rotateZone(z, ia.x1, ia.y1, cw, ch)
    return z
  })

  let drawingPreview = false
  if (ia?.mode === 'drawing') {
    const lx = Math.min(ia.x0, ia.x1), rx = Math.max(ia.x0, ia.x1)
    const ly = Math.min(ia.y0, ia.y1), ry = Math.max(ia.y0, ia.y1)
    const nx = snapEdge(lx / cw), nx1 = snapEdge(rx / cw)
    const ny = snapEdge(ly / ch), ny1 = snapEdge(ry / ch)
    if (nx1 - nx > 0.01 && ny1 - ny > 0.01) {
      display.push({ x: nx, y: ny, w: nx1 - nx, h: ny1 - ny, type: 'exclude' })
      drawingPreview = true
    }
  }

  for (let i = 0; i < display.length; i++) {
    const z = display[i]
    const isPreview = drawingPreview && i === display.length - 1
    const isSel = !isPreview && selIdx === i
    const [cx, cy] = zoneCenter(z, cw, ch)
    const hw = (z.w * cw) / 2, hh = (z.h * ch) / 2
    const a = deg2rad(z.rotation_deg ?? 0)
    const hex = z.color ?? (z.type === 'detect' ? '#f97316' : '#ef4444')
    const { r, g, b } = parseHex(hex)
    const fill = `rgba(${r},${g},${b},${isSel ? 0.30 : 0.18})`
    const stroke = `rgba(${r},${g},${b},${isSel ? 1.0 : 0.85})`
    const handle = `rgba(${r},${g},${b},0.95)`

    // Filled rotated rectangle
    ctx.save()
    ctx.translate(cx, cy)
    ctx.rotate(a)
    ctx.fillStyle = fill
    ctx.fillRect(-hw, -hh, hw * 2, hh * 2)
    ctx.strokeStyle = stroke
    ctx.lineWidth = isSel ? 2 : 1.5
    ctx.strokeRect(-hw, -hh, hw * 2, hh * 2)
    ctx.restore()

    if (!readonly && !isPreview) {
      // Corner handles at rotated positions
      for (let c = 0; c < 4; c++) {
        const [wx, wy] = cornerWorld(z, c, cw, ch)
        ctx.fillStyle = handle
        ctx.fillRect(wx - 5, wy - 5, 10, 10)
      }

      // Delete button just outside TR corner
      const [bx, by] = delBtnWorld(z, cw, ch)
      drawDeleteButton(ctx, bx, by, stroke)

      // Rotation handle (only for selected zone) — clamped so it's always on-screen
      if (isSel) {
        const [hx, hy] = rotHandleVisible(z, cw, ch)
        const [tx, ty] = toWorld(cx, cy, 0, -hh, a)  // rotated top-center
        ctx.beginPath()
        ctx.moveTo(tx, ty)
        ctx.lineTo(hx, hy)
        ctx.strokeStyle = handle
        ctx.lineWidth = 1.2
        ctx.stroke()
        ctx.beginPath()
        ctx.arc(hx, hy, ROT_R, 0, Math.PI * 2)
        ctx.fillStyle = handle
        ctx.fill()
      }

      // Label: drawn in zone's local frame so it rotates with the zone
      const label = z.label || (z.type === 'detect' ? 'detect' : null)
      if (label) {
        ctx.save()
        const [tlx, tly] = cornerWorld(z, 0, cw, ch)
        ctx.translate(tlx, tly)
        ctx.rotate(a)
        ctx.font = '11px sans-serif'
        ctx.textAlign = 'left'
        const tw = ctx.measureText(label).width
        ctx.fillStyle = 'rgba(0,0,0,0.65)'
        ctx.fillRect(4, 4, tw + 8, 16)
        ctx.fillStyle = stroke
        ctx.fillText(label, 8, 15)
        ctx.restore()
      }
    }
  }
}

// ── component ─────────────────────────────────────────────────────────────────

function relPos(e: React.MouseEvent<HTMLCanvasElement>): [number, number] {
  const c = e.currentTarget
  const r = c.getBoundingClientRect()
  return [
    clamp((e.clientX - r.left) * c.width / r.width, 0, c.width),
    clamp((e.clientY - r.top) * c.height / r.height, 0, c.height),
  ]
}

export default function CameraZonesSettingsPage() {
  const { id } = useParams<{ id: string }>()
  const isAdmin = getRole() === 'admin'
  const { settings } = useSettings()
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

  // Sync refs every render (no dep array — runs every render)
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
    } catch { /* ignore */ }
  }, [])

  useEventSource(sseURL, handleScoreMessage)

  // Load zones
  useEffect(() => {
    if (!id) return
    let active = true
    fetch(`/api/cameras/${id}/motion/zones`, { headers: authHeaders() })
      .then(r => r.json())
      .then((data: Zone[]) => { if (!active) return; setZones(data ?? []); setLoading(false) })
      .catch(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [id])

  // HLS live stream
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
        xhrSetup(xhr) { xhr.setRequestHeader('Authorization', `Bearer ${getToken()}`) },
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

  // RAF paint loop
  useEffect(() => {
    function loop() {
      const canvas = canvasRef.current
      const ctx = canvas?.getContext('2d')
      if (canvas && ctx) {
        paintCanvas(
          ctx, videoRef.current, zonesRef.current,
          canvas.width, canvas.height,
          interactionRef.current, selectedIdxRef.current, !isAdmin,
        )
      }
      rafRef.current = requestAnimationFrame(loop)
    }
    rafRef.current = requestAnimationFrame(loop)
    return () => cancelAnimationFrame(rafRef.current)
  }, [isAdmin])

  // ── cursor ──────────────────────────────────────────────────────────────────

  function updateCursor(px: number, py: number) {
    const canvas = canvasRef.current
    if (!canvas) return
    const cw = canvas.width, ch = canvas.height
    const zs = zonesRef.current
    const si = selectedIdxRef.current

    for (let i = zs.length - 1; i >= 0; i--) {
      if (hitDelBtn(zs[i], px, py, cw, ch)) { setCursorStyle('pointer'); return }
    }
    if (si !== null && zs[si] && hitRotHandle(zs[si], px, py, cw, ch)) {
      setCursorStyle('alias'); return
    }
    for (let i = zs.length - 1; i >= 0; i--) {
      const c = hitCorner(zs[i], px, py, cw, ch)
      if (c >= 0) { setCursorStyle(CORNER_CURSORS[c]); return }
    }
    for (let i = zs.length - 1; i >= 0; i--) {
      if (hitZone(zs[i], px, py, cw, ch)) { setCursorStyle('grab'); return }
    }
    setCursorStyle('crosshair')
  }

  // ── commit ──────────────────────────────────────────────────────────────────

  const commitInteraction = useCallback(() => {
    const ia = interactionRef.current
    const canvas = canvasRef.current
    if (!ia || !canvas) { setInteraction(null); return }
    interactionRef.current = null  // guard against double-commit (canvas + document)
    const cw = canvas.width, ch = canvas.height

    let next: Zone[] = zonesRef.current
    if (ia.mode === 'drawing') {
      const lx = Math.min(ia.x0, ia.x1), rx = Math.max(ia.x0, ia.x1)
      const ly = Math.min(ia.y0, ia.y1), ry = Math.max(ia.y0, ia.y1)
      const nx = snapEdge(lx / cw), nx1 = snapEdge(rx / cw)
      const ny = snapEdge(ly / ch), ny1 = snapEdge(ry / ch)
      if (nx1 - nx > 0.01 && ny1 - ny > 0.01) {
        const newZone: Zone = { x: nx, y: ny, w: nx1 - nx, h: ny1 - ny, type: 'exclude', color: pickZoneColor(next) }
        next = [...next, newZone]
        setSelectedIdx(next.length - 1)
        setRegionScore(null); setPeakScore(null)
      }
    } else if (ia.mode === 'moving') {
      next = next.map((z, i) => i === ia.idx ? moveZone(ia.orig, ia.x1 - ia.x0, ia.y1 - ia.y0, cw, ch) : z)
    } else if (ia.mode === 'resizing') {
      next = next.map((z, i) => i === ia.idx ? resizeZone(z, ia.corner, ia.x1, ia.y1, cw, ch) : z)
    } else if (ia.mode === 'rotating') {
      next = next.map((z, i) => i === ia.idx ? rotateZone(z, ia.x1, ia.y1, cw, ch) : z)
    }
    // Sync ref immediately so save() reads correct positions before next render
    zonesRef.current = next
    setZones(next)
    setInteraction(null)
  }, [])

  useEffect(() => {
    document.addEventListener('pointerup', commitInteraction)
    return () => document.removeEventListener('pointerup', commitInteraction)
  }, [commitInteraction])

  // ── mouse handlers ──────────────────────────────────────────────────────────

  function handleMouseDown(e: React.MouseEvent<HTMLCanvasElement>) {
    const [x, y] = relPos(e)
    const canvas = canvasRef.current
    if (!canvas) return
    const cw = canvas.width, ch = canvas.height
    const zs = zonesRef.current
    const si = selectedIdxRef.current

    // Delete button (checked first, front to back)
    for (let i = zs.length - 1; i >= 0; i--) {
      if (hitDelBtn(zs[i], x, y, cw, ch)) {
        setZones(prev => prev.filter((_, j) => j !== i))
        setSelectedIdx(prev => {
          if (prev === i) { setRegionScore(null); setPeakScore(null); return null }
          if (prev !== null && i < prev) return prev - 1
          return prev
        })
        return
      }
    }

    // Rotation handle (only for selected zone)
    if (si !== null && zs[si] && hitRotHandle(zs[si], x, y, cw, ch)) {
      setInteraction({ mode: 'rotating', idx: si, x1: x, y1: y })
      setCursorStyle('alias')
      return
    }

    // Corner resize handles (front to back)
    for (let i = zs.length - 1; i >= 0; i--) {
      const c = hitCorner(zs[i], x, y, cw, ch)
      if (c >= 0) {
        if (i !== si) { setRegionScore(null); setPeakScore(null) }
        setSelectedIdx(i)
        setInteraction({ mode: 'resizing', idx: i, corner: c, x1: x, y1: y })
        setCursorStyle(CORNER_CURSORS[c])
        return
      }
    }

    // Zone interior → move
    for (let i = zs.length - 1; i >= 0; i--) {
      if (hitZone(zs[i], x, y, cw, ch)) {
        if (i !== si) { setRegionScore(null); setPeakScore(null) }
        setSelectedIdx(i)
        setInteraction({ mode: 'moving', idx: i, orig: zs[i], x0: x, y0: y, x1: x, y1: y })
        setCursorStyle('grabbing')
        return
      }
    }

    // Empty area → draw
    setSelectedIdx(null)
    setRegionScore(null); setPeakScore(null)
    setInteraction({ mode: 'drawing', x0: x, y0: y, x1: x, y1: y })
  }

  function handleMouseMove(e: React.MouseEvent<HTMLCanvasElement>) {
    const [x, y] = relPos(e)
    if (interactionRef.current) {
      setInteraction(ia => ia ? { ...ia, x1: x, y1: y } : null)
    } else {
      updateCursor(x, y)
    }
  }

  function handleMouseUp(e: React.MouseEvent<HTMLCanvasElement>) {
    e.preventDefault()
    commitInteraction()
    const [x, y] = relPos(e)
    updateCursor(x, y)
  }

  function updateSelectedZone(patch: Partial<Zone>) {
    if (selectedIdx === null) return
    setZones(prev => prev.map((z, i) => i === selectedIdx ? { ...z, ...patch } : z))
  }

  async function save() {
    setSaving(true)
    try {
      const r = await fetch(`/api/cameras/${id}/motion/zones`, {
        method: 'PUT',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify(zonesRef.current),
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
      <CameraSettingsTabs id={id!} active="zones" camName={cam?.name} />

      {isAdmin && (
        <p className="text-xs text-gray-500 mb-5">
          Arraste em área vazia para criar uma zona. Clique numa zona para selecioná-la. Arraste os cantos para redimensionar. Use o círculo acima para rotacionar. Clique no × para excluir.
        </p>
      )}

      {isAdmin && !settings ? (
        <p className="text-gray-500 text-sm">Carregando...</p>
      ) : isAdmin && !cam ? (
        <p className="text-gray-500 text-sm">Câmera não encontrada.</p>
      ) : loading ? (
        <p className="text-gray-500 text-sm">Carregando zonas...</p>
      ) : (
        <div className="flex flex-col gap-4">
          <div
            className="relative bg-gray-900 border border-gray-800 rounded-lg overflow-hidden"
            style={{ aspectRatio: '16/9' }}
          >
            <canvas
              ref={canvasRef}
              width={1280}
              height={720}
              className="w-full h-full select-none"
              style={{ cursor: isAdmin ? cursorStyle : 'default' }}
              onMouseDown={isAdmin ? handleMouseDown : undefined}
              onMouseMove={isAdmin ? handleMouseMove : undefined}
              onMouseUp={isAdmin ? handleMouseUp : undefined}
              onMouseLeave={isAdmin ? () => setCursorStyle('crosshair') : undefined}
            />
          </div>

          {isAdmin && selectedZone && (
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
                    max={359}
                    step={1}
                    value={Math.round(selectedZone.rotation_deg ?? 0)}
                    onChange={e => {
                      let d = parseFloat(e.target.value) || 0
                      d = ((d % 360) + 360) % 360
                      updateSelectedZone({ rotation_deg: d })
                    }}
                    className="bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-sm text-gray-200 focus:outline-none focus:border-gray-600 w-28"
                  />
                </div>

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

          {isAdmin && toast && (
            <p className={`text-sm ${toast.ok ? 'text-green-400' : 'text-red-400'}`}>{toast.msg}</p>
          )}

          {isAdmin && (
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
          )}

          {!isAdmin && zones.length > 0 && (
            <p className="text-xs text-gray-600">{zones.length} zona{zones.length !== 1 ? 's' : ''}</p>
          )}
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
