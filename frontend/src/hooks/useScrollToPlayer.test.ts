import { describe, it, expect, vi } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useScrollToPlayer } from './useScrollToPlayer'

describe('useScrollToPlayer', () => {
  it('does not call scrollIntoView when key is null', () => {
    const el = { scrollIntoView: vi.fn() }
    const ref = { current: el as unknown as HTMLElement }

    renderHook(() => useScrollToPlayer(ref, null))

    expect(el.scrollIntoView).not.toHaveBeenCalled()
  })

  it('calls scrollIntoView when key changes from null to a value', () => {
    const el = { scrollIntoView: vi.fn() }
    const ref = { current: el as unknown as HTMLElement }

    const { rerender } = renderHook(({ key }: { key: string | null }) =>
      useScrollToPlayer(ref, key),
      { initialProps: { key: null } }
    )

    expect(el.scrollIntoView).not.toHaveBeenCalled()
    rerender({ key: 'rec-1' })
    expect(el.scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'start' })
  })

  it('calls scrollIntoView again when switching between recordings', () => {
    const el = { scrollIntoView: vi.fn() }
    const ref = { current: el as unknown as HTMLElement }

    const { rerender } = renderHook(({ key }: { key: string | null }) =>
      useScrollToPlayer(ref, key),
      { initialProps: { key: 'rec-1' } }
    )

    rerender({ key: 'rec-2' })
    expect(el.scrollIntoView).toHaveBeenCalledTimes(2)
  })

  it('does not scroll again when the same key is re-rendered', () => {
    const el = { scrollIntoView: vi.fn() }
    const ref = { current: el as unknown as HTMLElement }

    const { rerender } = renderHook(({ key }: { key: string | null }) =>
      useScrollToPlayer(ref, key),
      { initialProps: { key: 'rec-1' } }
    )

    rerender({ key: 'rec-1' })
    expect(el.scrollIntoView).toHaveBeenCalledTimes(1)
  })
})
