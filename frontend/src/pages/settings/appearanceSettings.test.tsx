import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import AppearanceSettingsPage from './AppearanceSettingsPage'

vi.mock('../../components/SettingsLayout', () => ({
  default: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}))
vi.mock('../../contexts/ThemeContext', () => ({
  useTheme: () => ({ theme: 'dark', setTheme: vi.fn() }),
}))
vi.mock('../../contexts/DisplayModeContext', () => ({
  useDisplayMode: () => ({ sidebar: 'icons-text', player: 'icons-text' }),
  useSetDisplayMode: () => vi.fn(),
}))

afterEach(cleanup)

describe('AppearanceSettingsPage', () => {
  it('rotula a seção do seletor de cores como "Estilo" (não "Tema")', () => {
    render(<AppearanceSettingsPage />)
    expect(screen.getByText('Estilo')).toBeTruthy()
    expect(screen.queryByText('Tema')).toBeNull()
  })
})
