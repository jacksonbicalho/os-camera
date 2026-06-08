import { describe, expect, it } from 'vitest'
import {
  IDENTITY,
  MAX_SCALE,
  clampState,
  isZoomed,
  panBy,
  transformStyle,
  zoomAtPoint,
} from './playerZoom'

const W = 1000
const H = 500

describe('playerZoom', () => {
  it('starts at identity (no transform)', () => {
    expect(IDENTITY).toEqual({ scale: 1, offsetX: 0, offsetY: 0 })
    expect(isZoomed(IDENTITY)).toBe(false)
    expect(transformStyle(IDENTITY)).toBe('translate(0px, 0px) scale(1)')
  })

  it('zoomAtPoint keeps the content point under the cursor fixed', () => {
    const cx = 800
    const cy = 100
    const s = zoomAtPoint(IDENTITY, 2, cx, cy, W, H)
    expect(s.scale).toBe(2)
    // content point under cursor before = (cx - offsetX)/scale; after must map back to cx
    const contentX = (cx - s.offsetX) / s.scale
    const contentY = (cy - s.offsetY) / s.scale
    expect(contentX).toBeCloseTo(cx) // identity content coords == screen coords at scale 1
    expect(contentY).toBeCloseTo(cy)
  })

  it('never lets the scaled video uncover the container (offset in [-(s-1)*size, 0])', () => {
    const s = zoomAtPoint(IDENTITY, 2, 0, 0, W, H) // zoom at top-left corner
    expect(s.offsetX).toBeLessThanOrEqual(0)
    expect(s.offsetX).toBeGreaterThanOrEqual(-(s.scale - 1) * W)
    expect(s.offsetY).toBeLessThanOrEqual(0)
    expect(s.offsetY).toBeGreaterThanOrEqual(-(s.scale - 1) * H)
  })

  it('clamps scale to [1, MAX_SCALE] and snaps offset back to 0 at scale 1', () => {
    const big = zoomAtPoint(IDENTITY, 999, W / 2, H / 2, W, H)
    expect(big.scale).toBe(MAX_SCALE)
    const out = clampState({ scale: 0.1, offsetX: -200, offsetY: -200 }, W, H)
    expect(out).toEqual({ scale: 1, offsetX: 0, offsetY: 0 })
  })

  it('panBy moves the offset and clamps to the edges', () => {
    const z = zoomAtPoint(IDENTITY, 2, W / 2, H / 2, W, H) // scale 2, centered
    const moved = panBy(z, 100, 50, W, H)
    expect(moved.offsetX).toBe(z.offsetX + 100 > 0 ? 0 : z.offsetX + 100)
    // dragging far past the edge clamps, never exposing a gap
    const far = panBy(z, 100000, 100000, W, H)
    expect(far.offsetX).toBe(0)
    expect(far.offsetY).toBe(0)
    const farNeg = panBy(z, -100000, -100000, W, H)
    expect(farNeg.offsetX).toBe(-(z.scale - 1) * W)
    expect(farNeg.offsetY).toBe(-(z.scale - 1) * H)
  })

  it('isZoomed is true only above scale 1', () => {
    expect(isZoomed({ scale: 1, offsetX: 0, offsetY: 0 })).toBe(false)
    expect(isZoomed({ scale: 1.5, offsetX: 0, offsetY: 0 })).toBe(true)
  })
})
