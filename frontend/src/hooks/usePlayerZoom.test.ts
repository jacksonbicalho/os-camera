import { describe, expect, it } from 'vitest'
import { renderHook } from '@testing-library/react'
import { usePlayerZoom } from './usePlayerZoom'

describe('usePlayerZoom', () => {
  it('reset é estável entre re-renders (contrato do efeito de reset na CameraPage)', () => {
    const video = document.createElement('video')
    const { result, rerender } = renderHook(() => usePlayerZoom(() => video))
    const first = result.current.reset
    rerender()
    rerender()
    expect(result.current.reset).toBe(first)
  })

  it('começa sem zoom (escala 1) e expõe a API', () => {
    const { result } = renderHook(() => usePlayerZoom(() => null))
    expect(result.current.isZoomed).toBe(false)
    expect(result.current.scale).toBe(1)
    expect(typeof result.current.reset).toBe('function')
    expect(typeof result.current.setContainer).toBe('function')
  })
})
