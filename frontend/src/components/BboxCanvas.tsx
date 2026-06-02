import { useCallback, useEffect, useRef, useState } from 'react'

export interface BboxRect {
  x: number
  y: number
  w: number
  h: number
  rotation_deg?: number
}

// ── geometry ──────────────────────────────────────────────────────────────────

function clamp(v: number, lo: number, hi: number) { return Math.max(lo, Math.min(hi, v)) }
function deg2rad(d: number) { return (((d ?? 0) % 360 + 360) % 360) * Math.PI / 180 }

function toWorld(cx: number, cy: number, lx: number, ly: number, a: number): [number, number] {
  return [cx + lx * Math.cos(a) - ly * Math.sin(a), cy + lx * Math.sin(a) + ly * Math.cos(a)]
}

function toLocal(cx: number, cy: number, wx: number, wy: number, a: number): [number, number] {
  const dx = wx - cx, dy = wy - cy
  return [dx * Math.cos(a) + dy * Math.sin(a), -dx * Math.sin(a) + dy * Math.cos(a)]
}

function boxCenter(b: BboxRect, cw: number, ch: number): [number, number] {
  return [(b.x + b.w / 2) * cw, (b.y + b.h / 2) * ch]
}

const CORNER_SIGNS: [number, number][] = [[-1, -1], [1, -1], [1, 1], [-1, 1]]
const OPP_CORNER = [2, 3, 0, 1]
const CORNER_CURSORS = ['nw-resize', 'ne-resize', 'se-resize', 'sw-resize']
const HANDLE_R = 6
const ROT_R = 8
const ROT_OFFSET = 26
const DEL_R = 9
const DEL_OFFSET = DEL_R + 2

function cornerWorld(b: BboxRect, c: number, cw: number, ch: number): [number, number] {
  const [cx, cy] = boxCenter(b, cw, ch)
  const hw = (b.w * cw) / 2, hh = (b.h * ch) / 2
  const a = deg2rad(b.rotation_deg ?? 0)
  const [sx, sy] = CORNER_SIGNS[c]
  return toWorld(cx, cy, sx * hw, sy * hh, a)
}

function rotHandleWorld(b: BboxRect, cw: number, ch: number): [number, number] {
  const [cx, cy] = boxCenter(b, cw, ch)
  const hh = (b.h * ch) / 2
  const a = deg2rad(b.rotation_deg ?? 0)
  return toWorld(cx, cy, 0, -(hh + ROT_OFFSET), a)
}

function rotHandleVisible(b: BboxRect, cw: number, ch: number): [number, number] {
  const [hx, hy] = rotHandleWorld(b, cw, ch)
  return [clamp(hx, ROT_R + 4, cw - ROT_R - 4), clamp(hy, ROT_R + 4, ch - ROT_R - 4)]
}

function delBtnWorld(b: BboxRect, cw: number, ch: number): [number, number] {
  const [cx, cy] = boxCenter(b, cw, ch)
  const hw = (b.w * cw) / 2, hh = (b.h * ch) / 2
  const a = deg2rad(b.rotation_deg ?? 0)
  const off = DEL_OFFSET / Math.SQRT2
  return toWorld(cx, cy, hw + off, -hh - off, a)
}

function hitDelBtn(b: BboxRect, px: number, py: number, cw: number, ch: number): boolean {
  const [bx, by] = delBtnWorld(b, cw, ch)
  return Math.hypot(px - bx, py - by) <= DEL_R + 6
}

function nearDelBtn(b: BboxRect, px: number, py: number, cw: number, ch: number): boolean {
  const [bx, by] = delBtnWorld(b, cw, ch)
  return Math.hypot(px - bx, py - by) <= DEL_R + 14
}

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

function hitBox(b: BboxRect, px: number, py: number, cw: number, ch: number): boolean {
  const [cx, cy] = boxCenter(b, cw, ch)
  const hw = (b.w * cw) / 2, hh = (b.h * ch) / 2
  const a = deg2rad(b.rotation_deg ?? 0)
  const [lx, ly] = toLocal(cx, cy, px, py, a)
  return Math.abs(lx) < hw && Math.abs(ly) < hh
}

function hitCorner(b: BboxRect, px: number, py: number, cw: number, ch: number): number {
  for (let c = 0; c < 4; c++) {
    const [wx, wy] = cornerWorld(b, c, cw, ch)
    if (Math.hypot(px - wx, py - wy) <= HANDLE_R + 3) return c
  }
  return -1
}

function hitRotHandle(b: BboxRect, px: number, py: number, cw: number, ch: number): boolean {
  const [hx, hy] = rotHandleVisible(b, cw, ch)
  return Math.hypot(px - hx, py - hy) <= ROT_R + 3
}

function moveBox(b: BboxRect, dx: number, dy: number, cw: number, ch: number): BboxRect {
  const [cx, cy] = boxCenter(b, cw, ch)
  const hw = (b.w * cw) / 2, hh = (b.h * ch) / 2
  const ncx = clamp(cx + dx, hw, cw - hw)
  const ncy = clamp(cy + dy, hh, ch - hh)
  return { ...b, x: (ncx - hw) / cw, y: (ncy - hh) / ch }
}

function resizeBox(b: BboxRect, c: number, mx: number, my: number, cw: number, ch: number): BboxRect {
  const [fx, fy] = cornerWorld(b, OPP_CORNER[c], cw, ch)
  const ncx = (fx + mx) / 2, ncy = (fy + my) / 2
  const a = deg2rad(b.rotation_deg ?? 0)
  const [lx, ly] = toLocal(ncx, ncy, fx, fy, a)
  const hw = Math.max(0.015 * cw, Math.abs(lx))
  const hh = Math.max(0.015 * ch, Math.abs(ly))
  const nx = clamp((ncx - hw) / cw, 0, 1)
  const ny = clamp((ncy - hh) / ch, 0, 1)
  const nw = clamp((2 * hw) / cw, 0.02, 1 - nx)
  const nh = clamp((2 * hh) / ch, 0.02, 1 - ny)
  return { ...b, x: nx, y: ny, w: nw, h: nh }
}

function rotateBox(b: BboxRect, mx: number, my: number, cw: number, ch: number): BboxRect {
  const [cx, cy] = boxCenter(b, cw, ch)
  let deg = Math.atan2(mx - cx, -(my - cy)) * 180 / Math.PI
  if (deg < 0) deg += 360
  return { ...b, rotation_deg: deg }
}

// ── interaction state ─────────────────────────────────────────────────────────

type Interaction =
  | null
  | { mode: 'drawing'; x0: number; y0: number; x1: number; y1: number }
  | { mode: 'moving'; orig: BboxRect; x0: number; y0: number; x1: number; y1: number }
  | { mode: 'resizing'; corner: number; x1: number; y1: number }
  | { mode: 'rotating'; x1: number; y1: number }

// ── paint ─────────────────────────────────────────────────────────────────────

function paintCanvas(
  ctx: CanvasRenderingContext2D,
  box: BboxRect | null,
  cw: number,
  ch: number,
  ia: Interaction,
  r: number, g: number, bl: number,
  readonly: boolean,
) {
  ctx.clearRect(0, 0, cw, ch)

  let display: BboxRect | null = box
  if (ia?.mode === 'moving' && box) display = moveBox(ia.orig, ia.x1 - ia.x0, ia.y1 - ia.y0, cw, ch)
  else if (ia?.mode === 'resizing' && box) display = resizeBox(box, ia.corner, ia.x1, ia.y1, cw, ch)
  else if (ia?.mode === 'rotating' && box) display = rotateBox(box, ia.x1, ia.y1, cw, ch)
  else if (ia?.mode === 'drawing') {
    const lx = Math.min(ia.x0, ia.x1), rx = Math.max(ia.x0, ia.x1)
    const ly = Math.min(ia.y0, ia.y1), ry = Math.max(ia.y0, ia.y1)
    if ((rx - lx) / cw > 0.005 && (ry - ly) / ch > 0.005) {
      display = { x: lx / cw, y: ly / ch, w: (rx - lx) / cw, h: (ry - ly) / ch }
    } else {
      display = null
    }
  }

  if (!display || display.w < 0.005 || display.h < 0.005) return

  const [cx, cy] = boxCenter(display, cw, ch)
  const hw = (display.w * cw) / 2, hh = (display.h * ch) / 2
  const a = deg2rad(display.rotation_deg ?? 0)
  const stroke = `rgb(${r},${g},${bl})`
  const handle = `rgba(${r},${g},${bl},0.95)`

  ctx.save()
  ctx.translate(cx, cy)
  ctx.rotate(a)
  ctx.fillStyle = `rgba(${r},${g},${bl},0.15)`
  ctx.fillRect(-hw, -hh, hw * 2, hh * 2)
  ctx.strokeStyle = stroke
  ctx.lineWidth = 2
  ctx.strokeRect(-hw, -hh, hw * 2, hh * 2)
  ctx.restore()

  if (readonly) return

  for (let c = 0; c < 4; c++) {
    const [wx, wy] = cornerWorld(display, c, cw, ch)
    ctx.fillStyle = handle
    ctx.fillRect(wx - 5, wy - 5, 10, 10)
  }

  const [hx, hy] = rotHandleVisible(display, cw, ch)
  const [tx, ty] = toWorld(cx, cy, 0, -hh, a)
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

  const [dbx, dby] = delBtnWorld(display, cw, ch)
  drawDeleteButton(ctx, dbx, dby, stroke)
}

// ── component ─────────────────────────────────────────────────────────────────

interface BboxCanvasProps {
  box: BboxRect | null
  onChange: (box: BboxRect | null) => void
  color?: [number, number, number]
  className?: string
  width?: number
  height?: number
  readonly?: boolean
}

function relPos(e: React.MouseEvent<HTMLCanvasElement>): [number, number] {
  const c = e.currentTarget
  const rc = c.getBoundingClientRect()
  return [
    clamp((e.clientX - rc.left) * c.width / rc.width, 0, c.width),
    clamp((e.clientY - rc.top) * c.height / rc.height, 0, c.height),
  ]
}

export default function BboxCanvas({
  box,
  onChange,
  color = [52, 211, 153],
  className,
  width = 1280,
  height = 720,
  readonly = false,
}: BboxCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const [interaction, setInteraction] = useState<Interaction>(null)
  const [cursor, setCursor] = useState(readonly ? 'default' : 'crosshair')
  const boxRef = useRef(box)
  const interactionRef = useRef(interaction)
  const [r, g, bl] = color

  // Sync refs every render
  useEffect(() => { boxRef.current = box })
  useEffect(() => { interactionRef.current = interaction })

  // RAF paint loop
  useEffect(() => {
    let raf: number
    function loop() {
      const canvas = canvasRef.current
      const ctx = canvas?.getContext('2d')
      if (canvas && ctx) {
        paintCanvas(ctx, boxRef.current, canvas.width, canvas.height, interactionRef.current, r, g, bl, readonly)
      }
      raf = requestAnimationFrame(loop)
    }
    raf = requestAnimationFrame(loop)
    return () => cancelAnimationFrame(raf)
  }, [r, g, bl, readonly])

  function updateCursor(px: number, py: number) {
    if (readonly) return
    const canvas = canvasRef.current
    if (!canvas) return
    const cw = canvas.width, ch = canvas.height
    const bx = boxRef.current
    if (!bx) { setCursor('crosshair'); return }
    if (hitDelBtn(bx, px, py, cw, ch)) { setCursor('pointer'); return }
    if (hitRotHandle(bx, px, py, cw, ch)) { setCursor('alias'); return }
    const c = hitCorner(bx, px, py, cw, ch)
    if (c >= 0) { setCursor(CORNER_CURSORS[c]); return }
    if (hitBox(bx, px, py, cw, ch)) { setCursor('grab'); return }
    setCursor('crosshair')
  }

  const commitInteraction = useCallback(() => {
    const ia = interactionRef.current
    const canvas = canvasRef.current
    if (!ia || !canvas) { setInteraction(null); return }
    interactionRef.current = null
    const cw = canvas.width, ch = canvas.height
    const bx = boxRef.current

    if (ia.mode === 'drawing') {
      const lx = Math.min(ia.x0, ia.x1), rx = Math.max(ia.x0, ia.x1)
      const ly = Math.min(ia.y0, ia.y1), ry = Math.max(ia.y0, ia.y1)
      if ((rx - lx) / cw > 0.01 && (ry - ly) / ch > 0.01) {
        onChange({ x: lx / cw, y: ly / ch, w: (rx - lx) / cw, h: (ry - ly) / ch })
      }
    } else if (ia.mode === 'moving' && bx) {
      onChange(moveBox(ia.orig, ia.x1 - ia.x0, ia.y1 - ia.y0, cw, ch))
    } else if (ia.mode === 'resizing' && bx) {
      onChange(resizeBox(bx, ia.corner, ia.x1, ia.y1, cw, ch))
    } else if (ia.mode === 'rotating' && bx) {
      onChange(rotateBox(bx, ia.x1, ia.y1, cw, ch))
    }
    setInteraction(null)
  }, [onChange])

  useEffect(() => {
    document.addEventListener('pointerup', commitInteraction)
    return () => document.removeEventListener('pointerup', commitInteraction)
  }, [commitInteraction])

  function handleMouseDown(e: React.MouseEvent<HTMLCanvasElement>) {
    if (readonly) return
    const [x, y] = relPos(e)
    const canvas = canvasRef.current
    if (!canvas) return
    const cw = canvas.width, ch = canvas.height
    const bx = boxRef.current

    if (bx) {
      if (hitDelBtn(bx, x, y, cw, ch)) {
        onChange(null)
        setCursor('crosshair')
        return
      }
      if (nearDelBtn(bx, x, y, cw, ch)) return
      if (hitRotHandle(bx, x, y, cw, ch)) {
        setInteraction({ mode: 'rotating', x1: x, y1: y })
        setCursor('alias')
        return
      }
      const c = hitCorner(bx, x, y, cw, ch)
      if (c >= 0) {
        setInteraction({ mode: 'resizing', corner: c, x1: x, y1: y })
        setCursor(CORNER_CURSORS[c])
        return
      }
      if (hitBox(bx, x, y, cw, ch)) {
        setInteraction({ mode: 'moving', orig: bx, x0: x, y0: y, x1: x, y1: y })
        setCursor('grabbing')
        return
      }
    }

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

  return (
    <canvas
      ref={canvasRef}
      width={width}
      height={height}
      className={className}
      style={{ cursor }}
      onMouseDown={handleMouseDown}
      onMouseMove={handleMouseMove}
      onMouseUp={handleMouseUp}
      onMouseLeave={() => !readonly && setCursor('crosshair')}
    />
  )
}
