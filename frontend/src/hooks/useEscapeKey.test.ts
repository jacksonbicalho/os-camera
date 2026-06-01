import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useEscapeKey } from './useEscapeKey'

describe('useEscapeKey', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('calls onEscape when Escape is pressed', () => {
    const onEscape = vi.fn()
    renderHook(() => useEscapeKey(onEscape))

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))

    expect(onEscape).toHaveBeenCalledTimes(1)
  })

  it('does not call onEscape for other keys', () => {
    const onEscape = vi.fn()
    renderHook(() => useEscapeKey(onEscape))

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }))
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown' }))

    expect(onEscape).not.toHaveBeenCalled()
  })

  it('does not call onEscape when disabled', () => {
    const onEscape = vi.fn()
    renderHook(() => useEscapeKey(onEscape, false))

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))

    expect(onEscape).not.toHaveBeenCalled()
  })

  it('removes listener on unmount', () => {
    const onEscape = vi.fn()
    const { unmount } = renderHook(() => useEscapeKey(onEscape))

    unmount()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))

    expect(onEscape).not.toHaveBeenCalled()
  })

  it('removes listener when disabled after being enabled', () => {
    const onEscape = vi.fn()
    const { rerender } = renderHook(
      ({ enabled }: { enabled: boolean }) => useEscapeKey(onEscape, enabled),
      { initialProps: { enabled: true } }
    )

    rerender({ enabled: false })
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))

    expect(onEscape).not.toHaveBeenCalled()
  })

  it('registers listener when enabled after being disabled', () => {
    const onEscape = vi.fn()
    const { rerender } = renderHook(
      ({ enabled }: { enabled: boolean }) => useEscapeKey(onEscape, enabled),
      { initialProps: { enabled: false } }
    )

    rerender({ enabled: true })
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))

    expect(onEscape).toHaveBeenCalledTimes(1)
  })
})
