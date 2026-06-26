import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import DatePicker from './DatePicker'

afterEach(cleanup)

describe('DatePicker', () => {
  it('mostra a data e alterna o popover', () => {
    render(<DatePicker id="dp" value={new Date(2026, 5, 15)} onChange={() => {}} />)
    expect(document.getElementById('dp')!.textContent).toContain('15/06/2026')
    expect(document.getElementById('dp-popover')).toBeNull()
    fireEvent.click(document.getElementById('dp')!)
    expect(document.getElementById('dp-popover')).toBeTruthy()
  })

  it('escolher um dia dispara onChange e fecha o popover', () => {
    const onChange = vi.fn()
    render(<DatePicker id="dp" value={new Date(2026, 5, 15)} onChange={onChange} />)
    fireEvent.click(document.getElementById('dp')!)
    fireEvent.click(screen.getByText('20'))
    expect(onChange).toHaveBeenCalled()
    expect(document.getElementById('dp-popover')).toBeNull()
  })

  it('com availableDays: dia sem conteúdo fica desabilitado, dia com conteúdo habilitado', () => {
    const onChange = vi.fn()
    render(
      <DatePicker id="dp" value={new Date(2026, 5, 15)} onChange={onChange} availableDays={['2026-06-20']} />,
    )
    fireEvent.click(document.getElementById('dp')!)

    // 15/06 não está no conjunto → clicar não dispara onChange
    fireEvent.click(screen.getByText('15'))
    expect(onChange).not.toHaveBeenCalled()

    // 20/06 está no conjunto → clicar dispara
    fireEvent.click(screen.getByText('20'))
    expect(onChange).toHaveBeenCalled()
  })
})
