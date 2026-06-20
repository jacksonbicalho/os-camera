import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/react'
import EventFilterChips from './EventFilterChips'

afterEach(cleanup)

describe('EventFilterChips', () => {
  it('renderiza os 5 chips com id estável', () => {
    render(<EventFilterChips value="todos" onChange={() => {}} />)
    for (const k of ['todos', 'movimento', 'pessoa', 'ia', 'estados']) {
      expect(document.getElementById(`event-chip-${k}`), k).toBeTruthy()
    }
  })

  it('clicar num chip dispara onChange com a categoria', () => {
    const onChange = vi.fn()
    render(<EventFilterChips value="todos" onChange={onChange} />)
    fireEvent.click(document.getElementById('event-chip-pessoa')!)
    expect(onChange).toHaveBeenCalledWith('pessoa')
  })

  it('exibe contagem por chip quando fornecida', () => {
    render(<EventFilterChips value="todos" onChange={() => {}} counts={{ pessoa: 3 }} />)
    expect(document.getElementById('event-chip-pessoa')!.textContent).toContain('3')
  })
})
