import { describe, it, expect } from 'vitest'
import { applyFrameStep, applySameChunkStep, frameStepTime, loadedMetadataSeek, mergeRecordings, parseDurationToMs, secondStepTarget } from './cameraUtils'
import type { Recording } from './cameraUtils'

function rec(filename: string): Recording {
  const ts = filename.replace('.mp4', '')
  return {
    filename,
    start: `2026-05-06T${ts.slice(8, 10)}:${ts.slice(10, 12)}:${ts.slice(12, 14)}Z`,
    url: `/recordings/${filename}`,
    is_recording: false,
  }
}

// ─── mergeRecordings ────────────────────────────────────────────────────────

describe('mergeRecordings', () => {
  it('returns fresh when hasMore is false (complete list)', () => {
    const prev = [rec('20260506120000.mp4'), rec('20260506110000.mp4')]
    const fresh = [rec('20260506120000.mp4')]
    expect(mergeRecordings(prev, fresh, 'desc', false)).toEqual(fresh)
  })

  it('removes recording deleted from page-1 range in desc order', () => {
    const r120 = rec('20260506120000.mp4')
    const r110 = rec('20260506110000.mp4')
    const r100 = rec('20260506100000.mp4')
    // prev has 3; fresh page 1 has r120 and r100 (r110 was deleted)
    // boundary in desc = r100 (oldest fresh); r110 is within that range → removed
    const result = mergeRecordings([r120, r110, r100], [r120, r100], 'desc', true)
    expect(result.map(r => r.filename)).not.toContain('20260506110000.mp4')
  })

  it('keeps beyond-page-1 recording in desc order (older than oldest fresh)', () => {
    const r120 = rec('20260506120000.mp4')
    const r110 = rec('20260506110000.mp4')
    const r090 = rec('20260506090000.mp4') // loaded via "load more", older than r110
    // fresh page 1 = [r120, r110]; r090 is beyond page 1
    const result = mergeRecordings([r120, r110, r090], [r120, r110], 'desc', true)
    expect(result.map(r => r.filename)).toContain('20260506090000.mp4')
  })

  it('removes recording deleted from page-1 range in asc order', () => {
    const r100 = rec('20260506100000.mp4')
    const r110 = rec('20260506110000.mp4')
    const r120 = rec('20260506120000.mp4')
    // prev has 3; fresh page 1 has r100 and r120 (r110 deleted)
    // boundary in asc = r120 (newest fresh); r110 is within that range → removed
    const result = mergeRecordings([r100, r110, r120], [r100, r120], 'asc', true)
    expect(result.map(r => r.filename)).not.toContain('20260506110000.mp4')
  })

  it('adds new recording not previously in prev', () => {
    const r120 = rec('20260506120000.mp4')
    const r110 = rec('20260506110000.mp4')
    const result = mergeRecordings([r120], [r120, r110], 'desc', false)
    expect(result.map(r => r.filename)).toContain('20260506110000.mp4')
  })

  it('updates is_recording flag from fresh result', () => {
    const old = { ...rec('20260506120000.mp4'), is_recording: true }
    const fresh = { ...rec('20260506120000.mp4'), is_recording: false }
    const result = mergeRecordings([old], [fresh], 'desc', false)
    expect(result[0].is_recording).toBe(false)
  })

  it('returns same prev reference when nothing changed', () => {
    const prev = [rec('20260506120000.mp4'), rec('20260506110000.mp4')]
    const fresh = [{ ...prev[0] }, { ...prev[1] }]
    const result = mergeRecordings(prev, fresh, 'desc', false)
    expect(result).toBe(prev)
  })

  it('returns new array when is_recording flag changes', () => {
    const prev = [{ ...rec('20260506120000.mp4'), is_recording: true }]
    const fresh = [{ ...rec('20260506120000.mp4'), is_recording: false }]
    const result = mergeRecordings(prev, fresh, 'desc', false)
    expect(result).not.toBe(prev)
    expect(result[0].is_recording).toBe(false)
  })
})

// ─── parseDurationToMs ──────────────────────────────────────────────────────

describe('parseDurationToMs', () => {
  it('"5m" → 300000', () => expect(parseDurationToMs('5m')).toBe(300_000))
  it('"30s" → 30000', () => expect(parseDurationToMs('30s')).toBe(30_000))
  it('"6s" → 6000', () => expect(parseDurationToMs('6s')).toBe(6_000))
  it('"2h" → 7200000', () => expect(parseDurationToMs('2h')).toBe(7_200_000))
  it('undefined → default 30000', () => expect(parseDurationToMs(undefined)).toBe(30_000))
  it('empty string → default 30000', () => expect(parseDurationToMs('')).toBe(30_000))
})

// ─── secondStepTarget (Ctrl+Shift+seta) ─────────────────────────────────────

describe('secondStepTarget', () => {
  const recs = [
    rec('20260506120000.mp4'), // 12:00:00
    rec('20260506120500.mp4'), // 12:05:00
    rec('20260506121000.mp4'), // 12:10:00
  ]

  it('avança 1s dentro do mesmo chunk', () => {
    expect(secondStepTarget(recs, '20260506120500.mp4', 5, 300, 1))
      .toEqual({ kind: 'same', time: 6 })
  })

  it('retrocede 1s dentro do mesmo chunk', () => {
    expect(secondStepTarget(recs, '20260506120500.mp4', 5, 300, -1))
      .toEqual({ kind: 'same', time: 4 })
  })

  it('avança além do fim → carrega o próximo chunk no overflow', () => {
    const r = secondStepTarget(recs, '20260506120500.mp4', 300, 300, 1)
    expect(r).toMatchObject({ kind: 'load', offsetSeconds: 1 })
    expect(r && r.kind === 'load' && r.rec.filename).toBe('20260506121000.mp4')
  })

  it('retrocede antes do início → carrega o chunk anterior perto do fim', () => {
    const r = secondStepTarget(recs, '20260506120500.mp4', 0, 300, -1)
    expect(r).toMatchObject({ kind: 'load', offsetSeconds: 1, fromEnd: true })
    expect(r && r.kind === 'load' && r.rec.filename).toBe('20260506120000.mp4')
  })

  it('avança no último chunk sem próximo → null', () => {
    expect(secondStepTarget(recs, '20260506121000.mp4', 300, 300, 1)).toBeNull()
  })

  it('retrocede no primeiro chunk sem anterior → null', () => {
    expect(secondStepTarget(recs, '20260506120000.mp4', 0, 300, -1)).toBeNull()
  })
})

// ─── applySameChunkStep (Ctrl+Shift+seta, mesmo chunk) ───────────────────────

describe('applySameChunkStep', () => {
  function fakeVideo() {
    const calls = { played: 0, paused: 0 }
    return {
      currentTime: 0,
      play() { calls.played++ },
      pause() { calls.paused++ },
      calls,
    }
  }

  it('posiciona o vídeo no tempo alvo', () => {
    const v = fakeVideo()
    applySameChunkStep(v, 42)
    expect(v.currentTime).toBe(42)
  })

  it('mantém pausado: nunca dá play, sempre pause', () => {
    const v = fakeVideo()
    applySameChunkStep(v, 10)
    expect(v.calls.played).toBe(0)
    expect(v.calls.paused).toBe(1)
  })
})

// ─── loadedMetadataSeek (seek + intenção de play ao carregar metadata) ───────

describe('loadedMetadataSeek', () => {
  it('pendingFromEnd → seek a partir do fim, reproduz por padrão', () => {
    expect(loadedMetadataSeek(300, 5, null, false)).toEqual({ seekTo: 295, shouldPlay: true })
  })

  it('pending normal → seek absoluto, reproduz por padrão', () => {
    expect(loadedMetadataSeek(300, null, 12, false)).toEqual({ seekTo: 12, shouldPlay: true })
  })

  it('sem pending → sem seek, reproduz por padrão', () => {
    expect(loadedMetadataSeek(300, null, null, false)).toEqual({ seekTo: null, shouldPlay: true })
  })

  it('stepPaused → não reproduz (atalho mantém parado ao cruzar chunk)', () => {
    expect(loadedMetadataSeek(300, null, 12, true)).toEqual({ seekTo: 12, shouldPlay: false })
  })

  it('fromEnd nunca fica negativo', () => {
    expect(loadedMetadataSeek(3, 10, null, false)).toEqual({ seekTo: 0, shouldPlay: true })
  })
})

// ─── frameStepTime (←/→ frame a frame) ──────────────────────────────────────

describe('applyFrameStep (serializa seeks + cruza chunk nas bordas)', () => {
  const fd = 1 / 30
  function fakeVideo(seeking: boolean, currentTime = 5, duration = 300) {
    return { currentTime, duration, seeking, paused: 0, pause() { this.paused++ } }
  }

  it('busy: ignora o passo enquanto está seeking (não mexe em currentTime nem pausa)', () => {
    const v = fakeVideo(true)
    expect(applyFrameStep(v, 300, fd, 1)).toEqual({ kind: 'busy' })
    expect(v.currentTime).toBe(5)
    expect(v.paused).toBe(0)
  })

  it('applied: avança um frame e pausa dentro do chunk', () => {
    const v = fakeVideo(false)
    expect(applyFrameStep(v, 300, fd, 1)).toEqual({ kind: 'applied' })
    expect(v.currentTime).toBeCloseTo(5 + fd, 5)
    expect(v.paused).toBe(1)
  })

  it('applied: retrocede um frame dentro do chunk', () => {
    const v = fakeVideo(false)
    applyFrameStep(v, 300, fd, -1)
    expect(v.currentTime).toBeCloseTo(5 - fd, 5)
  })

  it('cross-forward: ao chegar no fim do chunk sinaliza troca pro próximo', () => {
    const v = fakeVideo(false, 300, 300)
    const r = applyFrameStep(v, 300, fd, 1)
    expect(r.kind).toBe('cross-forward')
    expect((r as { overflow: number }).overflow).toBeCloseTo(fd, 5)
    expect(v.currentTime).toBe(300) // não mexe; quem troca é o chamador
  })

  it('cross-back: ao chegar no início do chunk sinaliza troca pro anterior', () => {
    const v = fakeVideo(false, 0, 300)
    const r = applyFrameStep(v, 300, fd, -1)
    expect(r.kind).toBe('cross-back')
    expect((r as { overflow: number }).overflow).toBeCloseTo(fd, 5)
    expect(v.currentTime).toBe(0)
  })
})

describe('frameStepTime', () => {
  const fd = 1 / 30

  it('avança um frame', () => {
    expect(frameStepTime(5, 300, fd, 1)).toBeCloseTo(5 + fd, 5)
  })

  it('retrocede um frame', () => {
    expect(frameStepTime(5, 300, fd, -1)).toBeCloseTo(5 - fd, 5)
  })

  it('satura no fim (não passa da duração)', () => {
    expect(frameStepTime(300, 300, fd, 1)).toBe(300)
  })

  it('satura no início (não fica negativo)', () => {
    expect(frameStepTime(0, 300, fd, -1)).toBe(0)
  })
})
