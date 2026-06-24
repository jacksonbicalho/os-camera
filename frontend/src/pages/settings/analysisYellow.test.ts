/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest'
import src from './AnalysisSettingsPage.tsx?raw'

// Guarda: o AnalysisSettingsPage não deve usar a rampa `yellow-*` crua, que não
// existe no @theme nem é remapeada no modo claro (fica ilegível no branco). Os
// avisos devem usar a rampa `amber` (remapeada) ou o token semântico `warning`.
describe('AnalysisSettingsPage — cores de aviso', () => {
  it('não usa nenhuma classe yellow-* (usa amber/warning)', () => {
    const matches = src.match(/\byellow-\w/g) ?? []
    expect(matches).toEqual([])
  })
})
