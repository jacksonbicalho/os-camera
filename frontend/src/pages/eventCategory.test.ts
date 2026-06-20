import { describe, expect, it } from 'vitest'
import { eventCategory, filterEventsByCategory, eventTitle } from './eventCategory'

describe('eventCategory', () => {
  it('sem label → movimento', () => {
    expect(eventCategory({})).toBe('movimento')
    expect(eventCategory({ label: '' })).toBe('movimento')
    expect(eventCategory({ label: '   ' })).toBe('movimento')
  })
  it('label pessoa/person → pessoa', () => {
    expect(eventCategory({ label: 'pessoa' })).toBe('pessoa')
    expect(eventCategory({ label: 'Pessoa com chapéu' })).toBe('pessoa')
    expect(eventCategory({ label: 'person' })).toBe('pessoa')
  })
  it('outro label de modelo → ia', () => {
    expect(eventCategory({ label: 'carro' })).toBe('ia')
    expect(eventCategory({ label: 'cachorro' })).toBe('ia')
  })
})

describe('eventTitle', () => {
  it('título por categoria', () => {
    expect(eventTitle({})).toBe('Movimento detectado')
    expect(eventTitle({ label: 'pessoa' })).toBe('Pessoa detectada')
    expect(eventTitle({ label: 'carro' })).toBe('carro')
  })
})

describe('filterEventsByCategory', () => {
  const evs = [
    { id: 1, label: '' },
    { id: 2, label: 'pessoa' },
    { id: 3, label: 'carro' },
    { id: 4, label: 'Pessoa detectada' },
  ]
  it('todos devolve tudo', () => {
    expect(filterEventsByCategory(evs, 'todos')).toHaveLength(4)
  })
  it('filtra por movimento', () => {
    expect(filterEventsByCategory(evs, 'movimento').map(e => e.id)).toEqual([1])
  })
  it('filtra por pessoa', () => {
    expect(filterEventsByCategory(evs, 'pessoa').map(e => e.id)).toEqual([2, 4])
  })
  it('filtra por ia', () => {
    expect(filterEventsByCategory(evs, 'ia').map(e => e.id)).toEqual([3])
  })
  it('estados (sem eventos de estado em motion_events) devolve vazio', () => {
    expect(filterEventsByCategory(evs, 'estados')).toHaveLength(0)
  })
})
