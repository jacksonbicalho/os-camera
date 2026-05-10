import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import SettingsLayout from '../../components/SettingsLayout'
import ConfirmDialog from '../../components/ConfirmDialog'
import { authHeaders } from '../../auth'
import { useSettings } from '../../hooks/useSettings'

interface Zone {
  x: number
  y: number
  w: number
  h: number
}

type ResizeHandle = 'nw' | 'ne' | 'se' | 'sw'

type Interaction =
  | { mode: 'drawing'; startX: number; startY: number; curX: number; curY: number }
  | { mode: 'moving'; idx: number; orig: Zone; startX: number; startY: number; curX: number; curY: number }
  | { mode: 'resizing'; idx: number; orig: Zone; handle: ResizeHandle; startX: number; startY: number; curX: number; curY: number }
  | null

const HANDLE_PX = 10   // hit radius for corner handles
const DELETE_PX = 12   // half-size of the × button hit area
const SNAP = 0.025     // snap to frame edge if within 2.5%

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
  return { x: nx, y: ny, w: nx1 - nx, h: ny1 - ny }
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
  return { x, y, w, h }
}

function applyMove(orig: Zone, dx: number, dy: number, cw: number, ch: number): Zone {
  const nx = clamp(orig.x + dx / cw, 0, 1 - orig.w)
  const ny = clamp(orig.y + dy / ch, 0, 1 - orig.h)
  return { ...orig, x: nx, y: ny }
}

// Returns corner handle hit, or null
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

// Returns zone index whose × button was clicked, or -1
function detectDeleteButton(zones: Zone[], px: number, py: number, cw: number, ch: number): number {
  for (let i = zones.length - 1; i >= 0; i--) {
    const z = zones[i]
    const bx = (z.x + z.w) * cw - DELETE_PX  // center-x of × button
    const by = z.y * ch + DELETE_PX           // center-y of × button
    if (Math.abs(px - bx) <= DELETE_PX && Math.abs(py - by) <= DELETE_PX) {
      return i
    }
  }
  return -1
}

// Returns zone index under cursor (for move), or -1
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

function drawDeleteButton(ctx: CanvasRenderingContext2D, cx: number, cy: number) {
  const r = 9
  ctx.beginPath()
  ctx.arc(cx, cy, r, 0, Math.PI * 2)
  ctx.fillStyle = 'rgba(30,30,30,0.85)'
  ctx.fill()
  ctx.strokeStyle = 'rgba(239,68,68,0.9)'
  ctx.lineWidth = 1.2
  ctx.stroke()
  // draw ×
  const d = 4
  ctx.beginPath()
  ctx.moveTo(cx - d, cy - d); ctx.lineTo(cx + d, cy + d)
  ctx.moveTo(cx + d, cy - d); ctx.lineTo(cx - d, cy + d)
  ctx.strokeStyle = 'rgba(239,68,68,0.95)'
  ctx.lineWidth = 1.8
  ctx.stroke()
}

function paintCanvas(
  ctx: CanvasRenderingContext2D,
  img: HTMLImageElement | null,
  zones: Zone[],
  cw: number,
  ch: number,
  ia: Interaction,
) {
  ctx.clearRect(0, 0, cw, ch)
  if (img?.complete && img.naturalWidth > 0) {
    ctx.drawImage(img, 0, 0, cw, ch)
  } else {
    ctx.fillStyle = '#1f2937'
    ctx.fillRect(0, 0, cw, ch)
    ctx.fillStyle = '#6b7280'
    ctx.font = '14px sans-serif'
    ctx.textAlign = 'center'
    ctx.fillText('Câmera indisponível', cw / 2, ch / 2)
  }

  const preview: Zone[] = zones.map((z, i) => {
    if (ia?.mode === 'moving' && ia.idx === i)
      return applyMove(ia.orig, ia.curX - ia.startX, ia.curY - ia.startY, cw, ch)
    if (ia?.mode === 'resizing' && ia.idx === i)
      return applyResize(ia.orig, ia.handle, ia.curX - ia.startX, ia.curY - ia.startY, cw, ch)
    return z
  })
  if (ia?.mode === 'drawing') {
    preview.push(dragToZone(ia.startX, ia.startY, ia.curX, ia.curY, cw, ch))
  }

  for (let i = 0; i < preview.length; i++) {
    const z = preview[i]
    const x = z.x * cw; const y = z.y * ch; const w = z.w * cw; const h = z.h * ch

    // fill + border
    ctx.fillStyle = 'rgba(239,68,68,0.18)'
    ctx.fillRect(x, y, w, h)
    ctx.strokeStyle = 'rgba(239,68,68,0.85)'
    ctx.lineWidth = 1.5
    ctx.strokeRect(x, y, w, h)

    // corner handles (only for committed zones, not the one being drawn)
    const isDrawing = ia?.mode === 'drawing' && i === preview.length - 1
    if (!isDrawing) {
      const corners: [number, number][] = [[x, y], [x + w, y], [x + w, y + h], [x, y + h]]
      for (const [cx, cy] of corners) {
        ctx.fillStyle = 'rgba(239,68,68,0.9)'
        ctx.fillRect(cx - 5, cy - 5, 10, 10)
      }
      // × button at top-right
      drawDeleteButton(ctx, x + w - DELETE_PX, y + DELETE_PX)
    }
  }
}

export default function CameraZonesSettingsPage() {
  const { id } = useParams<{ id: string }>()
  const settings = useSettings(`/settings/cameras/${id}/zones`)
  const cam = settings?.cameras.find(c => c.id === id)

  const [zones, setZones] = useState<Zone[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [toast, setToast] = useState<{ msg: string; ok: boolean } | null>(null)
  const [confirmClear, setConfirmClear] = useState(false)
  const [interaction, setInteraction] = useState<Interaction>(null)
  const [cursorStyle, setCursorStyle] = useState('crosshair')

  const canvasRef = useRef<HTMLCanvasElement>(null)
  const imgRef = useRef<HTMLImageElement | null>(null)

  const snapshotURL = useMemo(() => {
    if (!id) return null
    const hdrs = authHeaders() as Record<string, string>
    const token = hdrs['Authorization']?.replace('Bearer ', '') ?? ''
    return `/api/cameras/${id}/snapshot?token=${encodeURIComponent(token)}`
  }, [id])

  useEffect(() => {
    if (!id) return
    let active = true
    fetch(`/api/cameras/${id}/motion/zones`, { headers: authHeaders() })
      .then(r => r.json())
      .then((data: Zone[]) => { if (!active) return; setZones(data ?? []); setLoading(false) })
      .catch(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [id])

  const redraw = useCallback((currentZones: Zone[], ia: Interaction) => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return
    paintCanvas(ctx, imgRef.current, currentZones, canvas.width, canvas.height, ia)
  }, [])

  useEffect(() => { redraw(zones, interaction) }, [zones, interaction, redraw])

  function updateCursor(px: number, py: number, currentZones: Zone[]) {
    const canvas = canvasRef.current
    if (!canvas) return
    const cw = canvas.width; const ch = canvas.height
    const del = detectDeleteButton(currentZones, px, py, cw, ch)
    if (del >= 0) { setCursorStyle('pointer'); return }
    const h = detectHandle(currentZones, px, py, cw, ch)
    if (h) { setCursorStyle(HANDLE_CURSORS[h.handle]); return }
    const hit = hitTest(currentZones, px, py, cw, ch)
    setCursorStyle(hit >= 0 ? 'grab' : 'crosshair')
  }

  function handleMouseDown(e: React.MouseEvent<HTMLCanvasElement>) {
    const { x, y } = relPos(e)
    const canvas = canvasRef.current
    if (!canvas) return
    const cw = canvas.width; const ch = canvas.height

    // × button takes priority
    const delIdx = detectDeleteButton(zones, x, y, cw, ch)
    if (delIdx >= 0) {
      setZones(prev => prev.filter((_, i) => i !== delIdx))
      return
    }
    // corner handle → resize
    const h = detectHandle(zones, x, y, cw, ch)
    if (h) {
      setInteraction({ mode: 'resizing', idx: h.idx, orig: zones[h.idx], handle: h.handle, startX: x, startY: y, curX: x, curY: y })
      setCursorStyle(HANDLE_CURSORS[h.handle])
      return
    }
    // inside zone → move
    const idx = hitTest(zones, x, y, cw, ch)
    if (idx >= 0) {
      setInteraction({ mode: 'moving', idx, orig: zones[idx], startX: x, startY: y, curX: x, curY: y })
      setCursorStyle('grabbing')
      return
    }
    // empty area → draw
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
      if (z.w > 0.01 && z.h > 0.01) setZones(prev => [...prev, z])
    } else if (interaction.mode === 'moving') {
      const moved = applyMove(interaction.orig, interaction.curX - interaction.startX, interaction.curY - interaction.startY, cw, ch)
      setZones(prev => prev.map((z, i) => i === interaction.idx ? moved : z))
    } else if (interaction.mode === 'resizing') {
      const resized = applyResize(interaction.orig, interaction.handle, interaction.curX - interaction.startX, interaction.curY - interaction.startY, cw, ch)
      setZones(prev => prev.map((z, i) => i === interaction.idx ? resized : z))
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

  return (
    <SettingsLayout>
      <nav className="flex items-center gap-1.5 text-xs text-gray-500 mb-5">
        <Link to="/settings/cameras" className="hover:text-gray-300 transition-colors">Câmeras</Link>
        <span>/</span>
        <Link to={`/settings/cameras/${id}`} className="hover:text-gray-300 transition-colors">{id}</Link>
        <span>/</span>
        <span className="text-gray-300">Zonas de exclusão</span>
      </nav>

      <h2 className="text-lg font-semibold text-gray-200 mb-2">{id} — zonas de exclusão</h2>
      <p className="text-xs text-gray-500 mb-5">
        Arraste em área vazia para criar uma zona. Arraste uma zona para movê-la. Arraste os cantos para redimensionar. Clique no × para excluir.
      </p>

      {!settings ? (
        <p className="text-gray-500 text-sm">Carregando...</p>
      ) : !cam ? (
        <p className="text-gray-500 text-sm">Câmera não encontrada.</p>
      ) : loading ? (
        <p className="text-gray-500 text-sm">Carregando zonas...</p>
      ) : (
        <div className="flex flex-col gap-4">
          <div
            className="relative bg-gray-900 border border-gray-800 rounded-lg overflow-hidden"
            style={{ aspectRatio: '16/9' }}
          >
            {snapshotURL && (
              <img
                src={snapshotURL}
                alt=""
                className="hidden"
                onLoad={ev => { imgRef.current = ev.currentTarget; redraw(zones, interaction) }}
                onError={() => { imgRef.current = null; redraw(zones, interaction) }}
              />
            )}
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
              <span className="text-xs text-gray-600">{zones.length} zona{zones.length !== 1 ? 's' : ''} definida{zones.length !== 1 ? 's' : ''}</span>
            )}
          </div>
        </div>
      )}

      <ConfirmDialog
        open={confirmClear}
        title="Limpar todas as zonas"
        message="Todas as zonas de exclusão serão removidas. Esta ação não pode ser desfeita."
        confirmLabel="Limpar"
        danger
        onConfirm={() => { setZones([]); setConfirmClear(false) }}
        onCancel={() => setConfirmClear(false)}
      />
    </SettingsLayout>
  )
}
