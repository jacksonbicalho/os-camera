import { afterEach, describe, it, expect } from 'vitest'
import { cleanup, render } from '@testing-library/react'
import { Button } from './button'

afterEach(cleanup)

describe('Button (shadcn/ui)', () => {
  it('renderiza um <button> com o texto e a classe do tema (default)', () => {
    const { getByRole } = render(<Button>Salvar</Button>)
    const btn = getByRole('button', { name: 'Salvar' })
    expect(btn).toBeTruthy()
    expect(btn.className).toContain('bg-primary')
  })

  it('aplica a variante secondary', () => {
    const { getByRole } = render(<Button variant="secondary">X</Button>)
    expect(getByRole('button').className).toContain('bg-secondary')
  })

  it('asChild renderiza o elemento filho (Slot)', () => {
    const { getByRole } = render(<Button asChild><a href="/x">Link</a></Button>)
    expect(getByRole('link', { name: 'Link' })).toBeTruthy()
  })
})
