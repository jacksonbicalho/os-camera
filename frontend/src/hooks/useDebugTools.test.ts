import { describe, it, expect } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { useDebugTools } from './useDebugTools'

describe('useDebugTools', () => {
  it('começa com tudo desligado', () => {
    const { result } = renderHook(() => useDebugTools())
    expect(result.current.showDebug).toBe(false)
    expect(result.current.showDebugChart).toBe(false)
    expect(result.current.analyzeMode).toBe(false)
    expect(result.current.analyzeBox).toBeNull()
    expect(result.current.analyzeScore).toBeNull()
  })

  it('closeDebug fecha o painel e reseta os dois checkboxes + box/score', () => {
    const { result } = renderHook(() => useDebugTools())

    // estado típico: painel aberto, ambos checkboxes ligados, com box e score
    act(() => {
      result.current.setShowDebug(true)
      result.current.setShowDebugChart(true)
      result.current.setAnalyzeMode(true)
      result.current.setAnalyzeBox({ x: 0.1, y: 0.2, w: 0.3, h: 0.4 })
      result.current.setAnalyzeScore(42)
    })
    expect(result.current.analyzeMode).toBe(true)
    expect(result.current.showDebugChart).toBe(true)

    act(() => { result.current.closeDebug() })

    expect(result.current.showDebug).toBe(false)
    expect(result.current.showDebugChart).toBe(false)
    expect(result.current.analyzeMode).toBe(false)
    expect(result.current.analyzeBox).toBeNull()
    expect(result.current.analyzeScore).toBeNull()
  })
})
