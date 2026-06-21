import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import Sidebar from './Sidebar'

const setDisplayMode = vi.fn()

vi.mock('../auth', () => ({
  getRole: () => 'admin',
  getUsername: () => 'admin',
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
  useSetDisplayMode: () => setDisplayMode,
}))

vi.mock('./ThemeModeNav', () => ({
  default: () => <div id="theme-mode-nav" />,
}))

afterEach(() => { cleanup(); setDisplayMode.mockClear() })

function renderSidebar() {
  return render(
    <MemoryRouter>
      <Sidebar username="admin" />
    </MemoryRouter>,
  )
}

const LINK_ITEMS: Array<[string, string, string]> = [
  ['nav-live', '/', 'Ao vivo'],
  ['nav-recordings', '/recordings', 'Gravações'],
  ['nav-maps', '/maps', 'Mapa'],
  ['nav-devices', '/devices', 'Dispositivos'],
  ['nav-reports', '/reports', 'Relatórios'],
]

const FOLLOWS = Node.DOCUMENT_POSITION_FOLLOWING

describe('Sidebar — nav rail principal', () => {
  it('renderiza os itens de navegação novos como links rotulados com id e rota', () => {
    renderSidebar()
    for (const [id, to, label] of LINK_ITEMS) {
      const el = document.getElementById(id)
      expect(el, id).toBeTruthy()
      expect(el!.getAttribute('href'), `${id} href`).toBe(to)
      expect(el!.textContent, `${id} label`).toContain(label)
    }
  })

  it('o item de Mapa usa o rótulo singular "Mapa"', () => {
    renderSidebar()
    expect(document.getElementById('nav-maps')!.textContent?.trim()).toBe('Mapa')
  })

  it('não duplica "Usuários" na nav (fica só no flyout de Configurações)', () => {
    renderSidebar()
    expect(document.getElementById('nav-users')).toBeNull()
  })

  it('mantém Eventos (sino) e Configurações (flyout) com seus rótulos', () => {
    renderSidebar()
    const events = document.getElementById('sidebar-notifications')!
    const settings = document.getElementById('sidebar-settings')!
    expect(events.textContent).toContain('Eventos')
    expect(settings.textContent).toContain('Configurações')
  })

  it('Configurações fica na nav do topo (após Relatórios), fora do grupo inferior', () => {
    renderSidebar()
    const settings = document.getElementById('sidebar-settings')!
    const reports = document.getElementById('nav-reports')!
    const bottom = document.getElementById('sidebar-bottom')!
    // não está no grupo inferior
    expect(bottom.contains(settings)).toBe(false)
    // aparece depois de Relatórios no DOM
    expect(reports.compareDocumentPosition(settings) & FOLLOWS).toBeTruthy()
  })

  it('o grupo inferior contém apenas Recolher menu e o bloco de usuário', () => {
    renderSidebar()
    const bottom = document.getElementById('sidebar-bottom')!
    expect(bottom.contains(document.getElementById('sidebar-collapse'))).toBe(true)
    expect(bottom.contains(document.getElementById('sidebar-user'))).toBe(true)
    expect(bottom.contains(document.getElementById('sidebar-settings'))).toBe(false)
  })

  it('bloco de usuário exibe nome e papel', () => {
    renderSidebar()
    const user = document.getElementById('sidebar-user')!
    expect(user.textContent).toContain('admin')
    expect(user.textContent).toContain('Administrador')
  })

  it('"Recolher menu" alterna o modo da sidebar para compacto (persistido)', () => {
    renderSidebar()
    const btn = document.getElementById('sidebar-collapse')!
    expect(btn).toBeTruthy()
    fireEvent.click(btn)
    expect(setDisplayMode).toHaveBeenCalledWith('sidebar', 'icons-only')
  })
})
