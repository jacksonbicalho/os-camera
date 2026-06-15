import { useState } from 'react'
import { useTheme, resolveMode, type Mode } from '../contexts/ThemeContext'
import { ChevronRight, Check } from './Icons'

const MODE_OPTIONS: { value: Mode; label: string }[] = [
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' },
  { value: 'system', label: 'Sistema' },
]

// Seletor de modo para o popup de configurações do sidebar (engrenagem). O gatilho
// exibe o modo efetivo; clicar abre as opções num flyout à direita; selecionar aplica o
// modo na hora (setTheme), fecha a lista e o gatilho reflete a nova seleção.
//
// "Sistema" resolve para dark ou light conforme o SO — não é um estado visual próprio.
// Por isso o gatilho e o ✓ mostram sempre o concreto resolvido (Light/Dark): escolher
// Sistema destaca Dark ou Light de acordo com o SO.
// `onSelect` é chamado após aplicar o modo — usado pelo Sidebar para fechar também
// o popup de configurações pai (não só o flyout interno).
export default function ThemeModeNav({ onSelect }: { onSelect?: () => void }) {
  const { mode, setMode } = useTheme()
  const [open, setOpen] = useState(false)
  // Após selecionar, suprime o reabrir-por-hover enquanto o cursor segue sobre o
  // menu — só volta a abrir quando o mouse sai e entra de novo (ou clica no gatilho).
  const [dismissed, setDismissed] = useState(false)
  const effective = resolveMode(mode)
  const current = MODE_OPTIONS.find(o => o.value === effective) ?? MODE_OPTIONS[1]

  const select = (value: Mode) => {
    setMode(value)
    setOpen(false)
    setDismissed(true)
    onSelect?.()
  }

  return (
    <div
      id="theme-mode-nav"
      className="border-t border-border relative"
      onMouseEnter={() => { if (!dismissed) setOpen(true) }}
      onMouseLeave={() => { setOpen(false); setDismissed(false) }}
    >
      <button
        id="theme-nav-current"
        type="button"
        onClick={() => { setDismissed(false); setOpen(v => !v) }}
        className="flex items-center justify-between w-full px-3 py-2 text-sm text-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
      >
        <span>{current.label}</span>
        <ChevronRight className="w-4 h-4" />
      </button>

      {open && (
        <div
          id="theme-mode-flyout"
          className="absolute left-full top-0 w-40 bg-surface border border-border rounded shadow-lg z-50"
        >
          {MODE_OPTIONS.map(({ value, label }) => {
            const active = effective === value
            return (
              <button
                key={value}
                id={`theme-mode-${value}`}
                type="button"
                aria-current={active ? 'true' : undefined}
                onClick={() => select(value)}
                className={`flex items-center gap-2 w-full text-left px-3 py-2 text-sm transition-colors ${
                  active
                    ? 'bg-accent text-accent-foreground font-medium'
                    : 'text-foreground hover:bg-accent hover:text-accent-foreground'
                }`}
              >
                <Check className={`w-4 h-4 shrink-0 ${active ? 'opacity-100' : 'opacity-0'}`} />
                <span>{label}</span>
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}
