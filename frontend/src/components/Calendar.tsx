import type { ComponentProps, CSSProperties } from 'react'
import { DayPicker } from 'react-day-picker'
import { ptBR } from 'date-fns/locale'
import 'react-day-picker/style.css'

// Células menores que o default do react-day-picker para o calendário caber no
// painel de eventos (w-72) sem estourar largura/altura.
const FIT_VARS = {
  '--rdp-day-width': '2.1rem',
  '--rdp-day-height': '2.1rem',
  '--rdp-day_button-width': '2.1rem',
  '--rdp-day_button-height': '2.1rem',
} as CSSProperties

// Calendar — DayPicker unificado do app. Usa o estilo padrão do react-day-picker
// (style.css), o mesmo que o carrossel dos Estados tinha — preferência do
// navigator. Repassa as demais props do DayPicker.
export default function Calendar({ style, ...props }: ComponentProps<typeof DayPicker>) {
  return <DayPicker locale={ptBR} style={{ ...FIT_VARS, ...style }} {...props} />
}
