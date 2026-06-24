import { describe, expect, it } from 'vitest'
import { stateTitle, formatHistoryTime } from './statesHistory'

describe('stateTitle', () => {
  it('prefixa o classificador ao estado', () => {
    expect(stateTitle('Corredor', 'com pessoa saindo')).toBe('Corredor com pessoa saindo')
  })
  it('tolera nome/estado vazio sem espaço sobrando', () => {
    expect(stateTitle('', 'vazio')).toBe('vazio')
    expect(stateTitle('Corredor', '')).toBe('Corredor')
  })
})

describe('formatHistoryTime', () => {
  it('formata dd/MM/yyyy HH:mm:ss em 24h', () => {
    // sem Z → interpretado como horário local (determinístico no teste)
    expect(formatHistoryTime('2026-06-23T08:08:05')).toBe('23/06/2026 08:08:05')
    expect(formatHistoryTime('2026-06-23T17:05:09')).toBe('23/06/2026 17:05:09')
  })
})
