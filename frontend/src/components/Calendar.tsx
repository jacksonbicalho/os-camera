import type { ComponentProps } from 'react'
import { DayPicker } from 'react-day-picker'
import { ptBR } from 'date-fns/locale'
import 'react-day-picker/style.css'

// Calendar — DayPicker unificado do app. Usa o estilo padrão do react-day-picker
// (style.css), o mesmo que o carrossel dos Estados tinha — preferência do
// navigator. Repassa as demais props do DayPicker.
export default function Calendar(props: ComponentProps<typeof DayPicker>) {
  return <DayPicker locale={ptBR} {...props} />
}
