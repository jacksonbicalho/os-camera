import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { AlertProvider, useAlert } from './AlertContext'
import AlertBanner from '../components/AlertBanner'

afterEach(cleanup)

function Trigger() {
  const { showAlert } = useAlert()
  return (
    <>
      <button onClick={() => showAlert({ type: 'error', message: 'Falhou!' })}>err</button>
      <button onClick={() => showAlert({ type: 'success', message: 'Salvo!', durationMs: 3000 })}>ok</button>
    </>
  )
}

function renderApp() {
  return render(
    <AlertProvider>
      <AlertBanner />
      <Trigger />
    </AlertProvider>
  )
}

describe('AlertContext + AlertBanner', () => {
  it('shows an alert when triggered and removes it on close', () => {
    renderApp()
    expect(screen.queryByText('Falhou!')).toBeNull()

    fireEvent.click(screen.getByText('err'))
    expect(screen.getByText('Falhou!')).toBeTruthy()
    expect(screen.getByRole('alert')).toBeTruthy()

    fireEvent.click(screen.getByLabelText('Fechar alerta'))
    expect(screen.queryByText('Falhou!')).toBeNull()
  })

  it('auto-dismisses an alert after its durationMs', () => {
    vi.useFakeTimers()
    try {
      renderApp()
      act(() => {
        fireEvent.click(screen.getByText('ok'))
      })
      expect(screen.getByText('Salvo!')).toBeTruthy()

      act(() => {
        vi.advanceTimersByTime(3000)
      })
      expect(screen.queryByText('Salvo!')).toBeNull()
    } finally {
      vi.useRealTimers()
    }
  })

  it('stacks multiple alerts', () => {
    renderApp()
    fireEvent.click(screen.getByText('err'))
    fireEvent.click(screen.getByText('err'))
    expect(screen.getAllByText('Falhou!')).toHaveLength(2)
  })
})
