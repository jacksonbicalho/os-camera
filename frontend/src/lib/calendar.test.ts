import { describe, it, expect } from 'vitest'
import { calendarContent, dateKey } from './calendar'

describe('dateKey', () => {
  it('formata a data local como yyyy-MM-dd', () => {
    expect(dateKey(new Date(2026, 5, 20))).toBe('2026-06-20')
    expect(dateKey(new Date(2026, 0, 3))).toBe('2026-01-03')
  })
})

describe('calendarContent', () => {
  it('sem dias: set vazio e sem limites de navegação', () => {
    const cc = calendarContent([], new Date(2026, 5, 26))
    expect(cc.daySet.size).toBe(0)
    expect(cc.startMonth).toBeUndefined()
    expect(cc.endMonth).toBeUndefined()
  })

  it('com dias: set com as datas, startMonth no 1º mês com conteúdo e endMonth no mês de hoje', () => {
    const cc = calendarContent(['2026-04-10', '2026-06-20'], new Date(2026, 5, 26))
    expect(cc.daySet.has('2026-04-10')).toBe(true)
    expect(cc.daySet.has('2026-06-20')).toBe(true)
    expect(cc.daySet.has('2026-05-01')).toBe(false)
    expect(cc.startMonth).toEqual(new Date(2026, 3, 1))
    expect(cc.endMonth).toEqual(new Date(2026, 5, 1))
  })
})
