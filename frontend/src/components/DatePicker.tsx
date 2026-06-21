import { useState, useRef, useEffect } from 'react'
import { format } from 'date-fns'
import { ptBR } from 'date-fns/locale'
import Calendar from './Calendar'
import { Button } from './ui/button'
import { CalendarDays } from './Icons'
import { cn } from '@/lib/utils'

interface DatePickerProps {
  value: Date
  onChange: (d: Date) => void
  /** Desabilita datas futuras. */
  disableFuture?: boolean
  /** Abre o popover para cima (quando o gatilho está perto do rodapé). */
  openUp?: boolean
  /** Alinhamento horizontal do popover. */
  align?: 'left' | 'right'
  id?: string
}

// DatePicker — botão com a data + popover com o Calendar unificado (redesign do
// Escopo B). Substitui os popovers de data ad-hoc da timeline e dos Estados.
export default function DatePicker({ value, onChange, disableFuture, openUp, align = 'left', id }: DatePickerProps) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])

  return (
    <div className="relative" ref={ref}>
      <Button id={id} variant="outline" size="sm" className="tabular-nums gap-1.5" onClick={() => setOpen(o => !o)}>
        <CalendarDays className="w-3.5 h-3.5" />
        {format(value, 'dd/MM/yyyy', { locale: ptBR })}
      </Button>
      {open && (
        <div
          id={id ? `${id}-popover` : undefined}
          className={cn(
            'absolute z-30 bg-surface border border-border rounded-lg p-2 shadow-xl',
            align === 'right' ? 'right-0' : 'left-0',
            openUp ? 'bottom-full mb-1' : 'mt-1',
          )}
        >
          <Calendar
            mode="single"
            selected={value}
            defaultMonth={value}
            disabled={disableFuture ? { after: new Date() } : undefined}
            onSelect={(d) => { if (d) { onChange(d); setOpen(false) } }}
          />
        </div>
      )}
    </div>
  )
}
