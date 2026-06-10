import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useMarkActiveEventRead } from './useMarkActiveEventRead'

const markRead = vi.fn()

vi.mock('../contexts/NotificationContext', () => ({
  useNotifications: () => ({ markRead }),
}))

beforeEach(() => markRead.mockClear())

describe('useMarkActiveEventRead', () => {
  it('does not mark anything when there is no active event', () => {
    renderHook(() => useMarkActiveEventRead('cam1', null))
    expect(markRead).not.toHaveBeenCalled()
  })

  it('marks the active event read using the NotificationContext id format', () => {
    renderHook(() => useMarkActiveEventRead('cam1', '2026-06-10T18:00:00Z'))
    expect(markRead).toHaveBeenCalledWith('cam1-2026-06-10T18:00:00Z')
    expect(markRead).toHaveBeenCalledTimes(1)
  })

  it('marks each event as the active one changes (sequential playback)', () => {
    const { rerender } = renderHook(
      ({ t }: { t: string | null }) => useMarkActiveEventRead('cam1', t),
      { initialProps: { t: null as string | null } },
    )
    expect(markRead).not.toHaveBeenCalled()

    rerender({ t: '2026-06-10T18:00:00Z' })
    rerender({ t: '2026-06-10T18:05:00Z' })

    expect(markRead).toHaveBeenNthCalledWith(1, 'cam1-2026-06-10T18:00:00Z')
    expect(markRead).toHaveBeenNthCalledWith(2, 'cam1-2026-06-10T18:05:00Z')
    expect(markRead).toHaveBeenCalledTimes(2)
  })

  it('does not re-mark when the active event is unchanged across renders', () => {
    const { rerender } = renderHook(
      ({ t }: { t: string | null }) => useMarkActiveEventRead('cam1', t),
      { initialProps: { t: '2026-06-10T18:00:00Z' as string | null } },
    )
    rerender({ t: '2026-06-10T18:00:00Z' })
    expect(markRead).toHaveBeenCalledTimes(1)
  })
})
