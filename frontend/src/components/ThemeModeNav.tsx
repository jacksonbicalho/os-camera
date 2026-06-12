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
export default function ThemeModeNav() {
  const { mode, setMode } = useTheme()
  const [open, setOpen] = useState(false)
  const effective = resolveMode(mode)
  const current = MODE_OPTIONS.find(o => o.value === effective) ?? MODE_OPTIONS[1]

  const select = (value: Mode) => {
    setMode(value)
    setOpen(false)
  }

  return (
    <div className="border-t border-gray-700 relative">
      <button
        id="theme-nav-current"
        type="button"
        onClick={() => setOpen(v => !v)}
        className="flex items-center justify-between w-full px-3 py-2 text-sm text-gray-300 hover:bg-gray-700 hover:text-white transition-colors"
      >
        <span>{current.label}</span>
        <ChevronRight className="w-4 h-4" />
      </button>

      {open && (
        <div
          id="theme-mode-flyout"
          className="absolute left-full top-0 w-40 bg-gray-800 border border-gray-700 rounded shadow-lg z-50"
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
                    ? 'bg-gray-700 text-white font-medium'
                    : 'text-gray-300 hover:bg-gray-700 hover:text-white'
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
