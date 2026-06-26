import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import PageHeader from './PageHeader'

afterEach(cleanup)

describe('PageHeader', () => {
  it('renderiza o título com o id no wrapper; size padrão (page) usa text-2xl', () => {
    render(<PageHeader id="ph" title="Relatórios" />)
    const h = screen.getByRole('heading', { name: 'Relatórios' })
    expect(h.className).toContain('text-2xl')
    expect(document.getElementById('ph')).toBeTruthy()
  })

  it('size="section" usa text-h2', () => {
    render(<PageHeader title="Sobre" size="section" />)
    const h = screen.getByRole('heading', { name: 'Sobre' })
    expect(h.className).toContain('text-h2')
  })

  it('subtítulo e ações: presentes quando passados, ausentes quando não', () => {
    const { rerender } = render(<PageHeader title="T" />)
    expect(screen.queryByText('sub')).toBeNull()
    expect(screen.queryByText('Ação')).toBeNull()

    rerender(<PageHeader title="T" subtitle="sub" actions={<button>Ação</button>} />)
    expect(screen.getByText('sub')).toBeTruthy()
    expect(screen.getByText('Ação')).toBeTruthy()
  })
})
