import { describe, it, expect, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import React from 'react'
import { DisplayModeProvider, useDisplayMode, useSetDisplayMode } from './DisplayModeContext'

const STORAGE_KEY = 'ui-display-mode'

const wrapper = ({ children }: { children: React.ReactNode }) => (
  <DisplayModeProvider>{children}</DisplayModeProvider>
)

beforeEach(() => {
  localStorage.clear()
})

describe('DisplayModeContext', () => {
  it('defaults to icons-only for both sections', () => {
    const { result } = renderHook(() => useDisplayMode(), { wrapper })
    expect(result.current.sidebar).toBe('icons-only')
    expect(result.current.player).toBe('icons-only')
  })

  it('reads initial values from localStorage', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ sidebar: 'icons-text', player: 'text-only' }))
    const { result } = renderHook(() => useDisplayMode(), { wrapper })
    expect(result.current.sidebar).toBe('icons-text')
    expect(result.current.player).toBe('text-only')
  })

  it('persists sidebar mode to localStorage', () => {
    const { result } = renderHook(
      () => ({ mode: useDisplayMode(), set: useSetDisplayMode() }),
      { wrapper }
    )
    act(() => { result.current.set('sidebar', 'icons-text') })
    expect(result.current.mode.sidebar).toBe('icons-text')
    expect(JSON.parse(localStorage.getItem(STORAGE_KEY)!).sidebar).toBe('icons-text')
  })

  it('persists player mode to localStorage', () => {
    const { result } = renderHook(
      () => ({ mode: useDisplayMode(), set: useSetDisplayMode() }),
      { wrapper }
    )
    act(() => { result.current.set('player', 'text-only') })
    expect(result.current.mode.player).toBe('text-only')
    expect(JSON.parse(localStorage.getItem(STORAGE_KEY)!).player).toBe('text-only')
  })

  it('changing one section does not affect the other', () => {
    const { result } = renderHook(
      () => ({ mode: useDisplayMode(), set: useSetDisplayMode() }),
      { wrapper }
    )
    act(() => { result.current.set('sidebar', 'icons-text') })
    expect(result.current.mode.player).toBe('icons-only')
  })
})
