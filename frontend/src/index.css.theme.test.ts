import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

// Teste-guarda: a camada de cor do index.css deve usar a paleta Material/MUI padrão.
// Ancora os valores-chave; a validação visual completa (dark/light em todas as páginas)
// é do navigator. Os componentes não mudam — só os valores de cor (tokens + rampas cruas).
const css = readFileSync(resolve(process.cwd(), 'src/index.css'), 'utf8')

const has = (re: RegExp) => expect(css).toMatch(re)

describe('index.css — paleta Material UI', () => {
  it('primary é o blue Material #1976d2', () => {
    has(/--color-primary:\s*#1976d2/i)
  })

  it('fundo dark neutro Material (#121212) e fundo light Material (#f5f5f5)', () => {
    has(/--color-background:\s*#121212/i)
    has(/--color-background:\s*#f5f5f5/i)
  })

  it('rampa de cinza dark neutra Material (gray-950 = #121212)', () => {
    has(/--color-gray-950:\s*#121212/i)
  })

  it('accents Material nas rampas cruas (blue-600 #1976d2, red-600 #d32f2f)', () => {
    has(/--color-blue-600:\s*#1976d2/i)
    has(/--color-red-600:\s*#d32f2f/i)
  })
})
