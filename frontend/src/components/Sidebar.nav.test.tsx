import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import Sidebar from './Sidebar'

vi.mock('../auth', () => ({
  getRole: () => 'admin',
  authHeaders: () => ({}),
  clearToken: vi.fn(),
  onUnauthorized: vi.fn(),
}))

vi.mock('../contexts/NotificationContext', () => ({
  useNotifications: () => ({
    notifications: [], unreadCount: 0,
    markRead: vi.fn(), markSelectedRead: vi.fn(),
    remove: vi.fn(), removeAll: vi.fn(), removeSelected: vi.fn(),
    browserSupported: false, browserPermission: 'default', browserEnabled: false,
    enableBrowserNotifications: vi.fn(), disableBrowserNotifications: vi.fn(),
  }),
}))

vi.mock('../contexts/UserNotificationContext', () => ({
  useUserNotifications: () => ({ unreadCount: 0 }),
}))

vi.mock('../contexts/SidebarContext', () => ({
  useSidebarItems: () => [],
}))

vi.mock('../contexts/DisplayModeContext', () => ({
  useDisplayMode: () => ({ sidebar: 'icons-text' }),
}))

// ThemeModeNav usa useTheme — substituímos por um marcador com o mesmo id.
vi.mock('./ThemeModeNav', () => ({
  default: () => <div id="theme-mode-nav" />,
}))

afterEach(cleanup)

function renderSidebar() {
  return render(
    <MemoryRouter>
      <Sidebar username="admin" />
    </MemoryRouter>,
  )
}

const FOLLOWS = Node.DOCUMENT_POSITION_FOLLOWING

describe('Sidebar — reorganização da navegação', () => {
  it('Câmeras fica no topo (abaixo do sino), fora da seção inferior', () => {
    renderSidebar()
    const cameras = document.getElementById('sidebar-cameras')!
    const bell = document.getElementById('sidebar-notifications')!
    const bottom = document.getElementById('sidebar-bottom')!
    expect(cameras).toBeTruthy()
    // não está dentro da seção inferior (Configurações/Usuário)
    expect(bottom.contains(cameras)).toBe(false)
    // aparece depois do sino no DOM
    expect(bell.compareDocumentPosition(cameras) & FOLLOWS).toBeTruthy()
  })

  it('Estatísticas saiu da base e virou link /stats no flyout, entre color mode e Sobre', () => {
    renderSidebar()
    // o antigo item da base não existe mais
    expect(document.getElementById('sidebar-stats')).toBeNull()

    // abre o flyout de configurações
    fireEvent.click(document.getElementById('sidebar-settings')!)

    const statsLink = document.querySelector('a[href="/stats"]')!
    const themeNav = document.getElementById('theme-mode-nav')!
    const about = document.querySelector('a[href="/settings/about"]')!
    expect(statsLink).toBeTruthy()
    // ordem: color mode → /stats → Sobre
    expect(themeNav.compareDocumentPosition(statsLink) & FOLLOWS).toBeTruthy()
    expect(statsLink.compareDocumentPosition(about) & FOLLOWS).toBeTruthy()
  })
})
