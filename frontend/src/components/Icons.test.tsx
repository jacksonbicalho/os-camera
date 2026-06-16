/// <reference types="vite/client" />
import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render } from '@testing-library/react'
import iconsSource from './Icons.tsx?raw'
import { Play, Settings, X } from './Icons'

afterEach(cleanup)

// Guarda: os ícones são SVG inline, sem depender de lucide-react. A paridade
// visual depende de reproduzir os atributos base do lucide (viewBox 24, fill
// none, stroke currentColor, strokeWidth 2, round) e de repassar className/props.
describe('Icons — sem dependência lucide-react', () => {
  it('não importa de lucide-react (atribuição ISC em comentário é permitida)', () => {
    expect(iconsSource).not.toMatch(/from\s+["']lucide-react["']/)
    expect(iconsSource).not.toMatch(/import\b[^\n]*\blucide-react\b/)
  })

  it('renderiza SVGs com os atributos base do lucide e repassa className', () => {
    for (const Icon of [Settings, Play, X]) {
      const { container } = render(<Icon className="w-4 h-4" />)
      const svg = container.querySelector('svg')!
      expect(svg.getAttribute('viewBox')).toBe('0 0 24 24')
      expect(svg.getAttribute('fill')).toBe('none')
      expect(svg.getAttribute('stroke')).toBe('currentColor')
      expect(svg.getAttribute('stroke-width')).toBe('2')
      expect(svg.getAttribute('stroke-linecap')).toBe('round')
      expect(svg.getAttribute('stroke-linejoin')).toBe('round')
      expect(svg.getAttribute('class')).toContain('w-4 h-4')
      cleanup()
    }
  })
})
