import { format } from 'date-fns'

// stateTitle compõe o título de uma transição no histórico: "<classificador> <estado>"
// (ex.: "Corredor com pessoa saindo"). Tolera nome ou estado vazio.
export function stateTitle(classifierName: string, state: string): string {
  return `${classifierName} ${state}`.trim()
}

// formatHistoryTime formata um timestamp ISO como dd/MM/yyyy HH:mm:ss (24h), no fuso local.
export function formatHistoryTime(iso: string): string {
  return format(new Date(iso), 'dd/MM/yyyy HH:mm:ss')
}
