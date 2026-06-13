import SettingsLayout from '../../components/SettingsLayout'
import { useDisplayMode, useSetDisplayMode, type DisplayMode } from '../../contexts/DisplayModeContext'
import { useTheme, type Mode } from '../../contexts/ThemeContext'

const THEME_OPTIONS: { value: Mode; label: string }[] = [
  { value: 'dark', label: 'Dark' },
  { value: 'light', label: 'Light' },
  { value: 'system', label: 'Sistema' },
]

const OPTIONS: { value: DisplayMode; label: string }[] = [
  { value: 'icons-only', label: 'Apenas ícones' },
  { value: 'icons-text', label: 'Ícones e textos' },
  { value: 'text-only',  label: 'Apenas textos' },
]

function ModeRadioGroup({
  value,
  onChange,
}: {
  value: DisplayMode
  onChange: (v: DisplayMode) => void
}) {
  return (
    <div className="flex gap-3 flex-wrap">
      {OPTIONS.map(opt => (
        <label key={opt.value} className="flex items-center gap-2 cursor-pointer select-none group">
          <input
            type="radio"
            name={undefined}
            checked={value === opt.value}
            onChange={() => onChange(opt.value)}
            className="accent-blue-500 cursor-pointer"
          />
          <span className="text-sm text-gray-300 group-hover:text-white transition-colors">
            {opt.label}
          </span>
        </label>
      ))}
    </div>
  )
}

export default function AppearanceSettingsPage() {
  const mode = useDisplayMode()
  const set = useSetDisplayMode()
  const { mode: colorMode, setMode } = useTheme()

  return (
    <SettingsLayout>
      <h3 className="text-h2 font-semibold text-gray-200">Aparência</h3>
      <p className="text-sm text-gray-500 mt-1 mb-6">Controla como botões e rótulos são exibidos na interface.</p>

      <div className="flex flex-col gap-6">
        <div className="bg-gray-900 border border-gray-800 rounded-lg p-5 flex flex-col gap-3">
          <div>
            <p className="text-sm font-medium text-gray-200">Estilo</p>
            <p className="text-xs text-gray-500 mt-0.5">Esquema de cores da interface.</p>
          </div>
          <div className="flex gap-3 flex-wrap">
            {THEME_OPTIONS.map(opt => (
              <label key={opt.value} className="flex items-center gap-2 cursor-pointer select-none group">
                <input
                  type="radio"
                  checked={colorMode === opt.value}
                  onChange={() => setMode(opt.value)}
                  className="accent-blue-500 cursor-pointer"
                />
                <span className="text-sm text-gray-300 group-hover:text-white transition-colors">
                  {opt.label}
                </span>
              </label>
            ))}
          </div>
        </div>

        <div className="bg-gray-900 border border-gray-800 rounded-lg p-5 flex flex-col gap-3">
          <div>
            <p className="text-sm font-medium text-gray-200">Sidebar</p>
            <p className="text-xs text-gray-500 mt-0.5">Botões e itens da barra lateral esquerda.</p>
          </div>
          <ModeRadioGroup value={mode.sidebar} onChange={v => set('sidebar', v)} />
        </div>

        <div className="bg-gray-900 border border-gray-800 rounded-lg p-5 flex flex-col gap-3">
          <div>
            <p className="text-sm font-medium text-gray-200">Topo do player</p>
            <p className="text-xs text-gray-500 mt-0.5">Controles acima do vídeo (mudo, velocidade, gravações…).</p>
          </div>
          <ModeRadioGroup value={mode.player} onChange={v => set('player', v)} />
        </div>
      </div>
    </SettingsLayout>
  )
}
