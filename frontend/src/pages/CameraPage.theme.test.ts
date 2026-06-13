import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

// Teste-guarda: a chrome da CameraPage deve usar os papéis semânticos do tema
// (bg-background/surface, text-foreground/muted/faint, border-border), não a
// rampa de cinza crua. Overlays do player (bg-black/text-white sobre vídeo) e
// accents vívidos (bg-blue-600, text-emerald-300…) são exceções — fora deste teto.
const source = readFileSync(
  resolve(process.cwd(), 'src/pages/CameraPage.tsx'),
  'utf8',
)

// Captura qualquer utilitário Tailwind sobre a rampa de cinza:
// bg-gray-700, text-gray-400, border-gray-800, hover:bg-gray-700, ring-gray-700…
const GRAY_RAMP = /\b[a-z-]*(?:bg|text|border|ring|divide|from|to|via)-gray-\d{2,3}\b/g

describe('CameraPage — migração para tokens de tema', () => {
  it('não usa nenhuma classe da rampa de cinza crua (*-gray-*)', () => {
    const matches = source.match(GRAY_RAMP) ?? []
    expect(matches).toEqual([])
  })

  // Letterbox do vídeo deve ser theme-aware (bg-background), não preto puro — senão
  // no modo light sobram barras pretas ao redor do vídeo. Overlays/scrims translúcidos
  // (bg-black/70, bg-black/60…) seguem corretos sobre vídeo e são preservados.
  it('não usa bg-black sólido no letterbox (só bg-black/NN translúcido é permitido)', () => {
    const matches = source.match(/bg-black(?!\/)/g) ?? []
    expect(matches).toEqual([])
  })
})
