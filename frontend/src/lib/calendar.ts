// Helpers de calendário compartilhados entre o componente DatePicker e as
// páginas que usam o Calendar direto (ex.: CameraPage). Derivam, a partir das
// datas com conteúdo, o conjunto de dias e os limites de navegação.

// dateKey formata uma Date como yyyy-MM-dd no fuso local — a chave usada para
// casar com as datas de "dias com conteúdo" vindas do backend.
export function dateKey(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

export interface CalendarContent {
  /** Datas (yyyy-MM-dd) que têm conteúdo — usadas para habilitar/destacar dias. */
  daySet: Set<string>
  /** 1º mês navegável (mês da 1ª data com conteúdo). undefined = sem limite. */
  startMonth?: Date
  /** Último mês navegável (mês de hoje). undefined = sem limite. */
  endMonth?: Date
}

// calendarContent deriva, a partir das datas com conteúdo, o conjunto de dias e
// os limites de navegação do calendário: o passado é limitado ao 1º mês com
// conteúdo; o futuro, ao mês de hoje (hoje precisa permanecer alcançável mesmo
// sem conteúdo — é a seleção padrão/ao vivo). Sem datas, não impõe limites.
export function calendarContent(days: string[], today: Date): CalendarContent {
  const daySet = new Set(days)
  if (days.length === 0) return { daySet }
  const first = parseDayKey([...days].sort()[0])
  return {
    daySet,
    startMonth: new Date(first.getFullYear(), first.getMonth(), 1),
    endMonth: new Date(today.getFullYear(), today.getMonth(), 1),
  }
}

export function parseDayKey(s: string): Date {
  const [y, m, d] = s.split('-').map(Number)
  return new Date(y, m - 1, d)
}
