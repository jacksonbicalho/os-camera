import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

// Teste-guarda: o VerticalTimeline (usado só na CameraPage) deve acompanhar o tema.
// A rampa `zinc` NÃO é remapeada pelo bloco legado de light (só `gray-*`/`white`), então
// `bg-zinc-950`/`border-zinc-800` deixariam o timeline preto no modo claro. Os segmentos
// de gravação não podem usar hex escuro fixo (#1f2937/#1e3a5f) — devem ser theme-aware.
// Accents (amber, #f97316 motion, #dc2626 ponteiro, rgba vermelho) seguem vívidos.
const source = readFileSync(
  resolve(process.cwd(), 'src/components/VerticalTimeline.tsx'),
  'utf8',
)

describe('VerticalTimeline — coerência de tema', () => {
  it('não usa a rampa zinc (não theme-aware no light)', () => {
    const matches = source.match(/\b[a-z-]*zinc-\d{2,3}\b/g) ?? []
    expect(matches).toEqual([])
  })

  it('não usa hex escuro fixo nos segmentos de gravação (#1f2937/#1e3a5f)', () => {
    const matches = source.match(/#1f2937|#1e3a5f/gi) ?? []
    expect(matches).toEqual([])
  })
})
