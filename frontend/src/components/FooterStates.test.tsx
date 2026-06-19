import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import FooterStates from './FooterStates'
import type { FooterState } from '../hooks/useFooterStates'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.useRealTimers()
})

function okJSON(data: FooterState[]) {
  return { ok: true, status: 200, json: async () => data }
}

describe('FooterStates', () => {
  it('renders nothing when the user has no footer classifiers', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(okJSON([])))
    const { container } = render(<FooterStates />)
    await waitFor(() => expect(container.querySelector('#footer-states')).toBeNull())
  })

  it('lists name: state from the endpoint', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      okJSON([{ classifier_id: 7, camera_id: 'cam1', name: 'Corredor', state: 'vazio' }]),
    ))
    render(<FooterStates />)
    expect(await screen.findByText('Corredor:')).toBeTruthy()
    expect(screen.getByText('vazio')).toBeTruthy()
  })

  it('flashes a classifier when its state changes between polls', async () => {
    vi.useFakeTimers()
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(okJSON([{ classifier_id: 7, camera_id: 'cam1', name: 'Corredor', state: 'vazio' }]))
      .mockResolvedValue(okJSON([{ classifier_id: 7, camera_id: 'cam1', name: 'Corredor', state: 'cheio' }]))
    vi.stubGlobal('fetch', fetchMock)

    render(<FooterStates />)
    // resolve a 1ª busca (mount)
    await act(async () => { await Promise.resolve() })

    const item = () => document.getElementById('footer-state-7')
    expect(item()?.style.animation).toBe('')

    // dispara o poll (5 s) → estado muda para "cheio" → pisca
    await act(async () => { await vi.advanceTimersByTimeAsync(5000) })
    expect(item()?.style.animation).toContain('footer-state-flash')

    // após ~1 s o flash some
    await act(async () => { await vi.advanceTimersByTimeAsync(1000) })
    expect(item()?.style.animation).toBe('')
  })
})
