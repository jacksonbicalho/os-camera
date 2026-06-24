import { useState, useRef, useEffect, Fragment } from "react"
import { createPortal } from "react-dom"
import { useNavigate, useLocation, Link, NavLink } from "react-router-dom"
import { clearToken, getRole } from "../auth"
import { useNotifications } from "../contexts/NotificationContext"
import { useUserNotifications } from "../contexts/UserNotificationContext"
import { useSidebarItems, type SidebarItem, type SidebarDropdownOption } from "../contexts/SidebarContext"
import { useDisplayMode, useSetDisplayMode } from "../contexts/DisplayModeContext"
import { formatDistanceToNow } from "date-fns"
import { ptBR } from "date-fns/locale"
import type { Notification } from "../contexts/NotificationContext"
import ConfirmDialog from "./ConfirmDialog"
import EventsPanelHeader from "./EventsPanelHeader"
import ThemeModeNav from "./ThemeModeNav"
import {
  Bell, X, Check, Settings, CircleUser, CameraLogo, Cctv,
  Film, BarChart2, ChevronLeft,
} from "./Icons"
import { Button, buttonVariants } from "./ui/button"
import { cn } from "@/lib/utils"

interface SidebarProps {
  username?: string
}

interface ConfirmState {
  title: string
  message: string
  confirmLabel?: string
  danger?: boolean
  action: () => void
}

// Itens de rota da nav rail principal (mockup do redesign). Os destinos
// Mapas/Dispositivos/Usuários/Relatórios são páginas placeholder por enquanto
// — preenchidas nas histórias seguintes do roadmap.
// O sino "Eventos" é renderizado antes destes (1º item do nav); ver o JSX.
const NAV_LINKS: Array<{ id: string; to: string; label: string; icon: React.ReactNode }> = [
  { id: "nav-live", to: "/", label: "Ao vivo", icon: <Cctv /> },
  { id: "nav-recordings", to: "/recordings", label: "Gravações", icon: <Film /> },
  { id: "nav-reports", to: "/reports", label: "Relatórios", icon: <BarChart2 /> },
]

function useDropdown(extraRef?: React.RefObject<HTMLElement | null>) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      const t = e.target as Node
      const inside = (ref.current?.contains(t) ?? false) || (extraRef?.current?.contains(t) ?? false)
      if (!inside) setOpen(false)
    }
    document.addEventListener("mousedown", handleClick)
    return () => document.removeEventListener("mousedown", handleClick)
  }, [extraRef])
  return { open, setOpen, ref }
}

function NotificationItem({
  n,
  checked,
  onToggle,
  onClick,
  onRemove,
}: {
  n: Notification
  checked: boolean
  onToggle: () => void
  onClick: () => void
  onRemove: () => void
}) {
  const relTime = formatDistanceToNow(new Date(n.time), {
    addSuffix: true,
    locale: ptBR,
  })

  return (
    <div
      className={`flex items-start gap-2 px-3 py-2 hover:bg-accent transition-colors ${
        !n.read ? "border-l-2 border-blue-500" : "border-l-2 border-transparent"
      }`}
    >
      <input
        type="checkbox"
        checked={checked}
        onChange={onToggle}
        onClick={(e) => e.stopPropagation()}
        className="mt-0.5 w-3 h-3 flex-shrink-0 accent-blue-500 cursor-pointer"
      />
      <div className="flex-1 min-w-0 cursor-pointer" onClick={onClick}>
        <div className="text-xs text-gray-300 font-medium truncate">
          {n.cameraName || n.cameraId}
        </div>
        <div className="text-xs text-gray-400">
          {n.label && <span style={{ color: n.color ?? "#f97316" }}>{n.label} · </span>}
          {(n.score * 100).toFixed(1)}% · {relTime}
        </div>
      </div>
      <Button
        variant="ghost"
        size="icon"
        onClick={(e) => { e.stopPropagation(); onRemove() }}
        className="h-6 w-6 shrink-0 mt-0.5 text-muted-foreground [&_svg]:size-3.5"
        title="Excluir"
      >
        <X />
      </Button>
    </div>
  )
}

function SidebarInjectedItem({ item, displayMode }: {
  item: SidebarItem
  displayMode: 'icons-only' | 'icons-text' | 'text-only'
}) {
  const [dropOpen, setDropOpen] = useState(false)
  const dropRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (item.type !== 'dropdown') return
    function handleClick(e: MouseEvent) {
      if (dropRef.current && !dropRef.current.contains(e.target as Node))
        setDropOpen(false)
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [item.type])

  const showIcon = displayMode !== 'text-only'
  const showLabel = displayMode !== 'icons-only'
  const base = showLabel
    ? `relative flex items-center gap-2 w-full px-3 h-9 rounded-lg transition-colors`
    : `relative flex items-center justify-center w-10 h-10 rounded-lg transition-colors`

  if (item.type === 'separator') {
    return <div className={showLabel ? 'w-full border-t border-border my-1' : 'w-8 border-t border-border my-1'} />
  }

  if (item.type === 'dropdown') {
    const isActive = item.active || dropOpen
    return (
      <div className={showLabel ? 'relative w-full' : 'relative'} ref={dropRef}>
        <button
          onClick={() => { if (!item.disabled) setDropOpen(v => !v) }}
          disabled={item.disabled}
          title={item.title}
          className={`${base} ${isActive ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'} ${item.disabled ? 'opacity-40 cursor-not-allowed' : ''}`}
        >
          {showIcon && item.icon}
          {showLabel && <span className="text-sm truncate">{item.title}</span>}
        </button>
        {dropOpen && (
          <div className="absolute left-full top-0 ml-2 bg-surface border border-border rounded-lg shadow-xl z-50 py-1 min-w-[72px]">
            {item.options.map((opt: SidebarDropdownOption, i: number) => (
              <button
                key={i}
                onClick={() => { opt.onClick(); setDropOpen(false) }}
                disabled={opt.disabled}
                className={`flex items-center justify-between gap-2 w-full px-3 py-1.5 text-sm text-left transition-colors ${
                  opt.active
                    ? 'text-blue-400 font-semibold'
                    : 'text-foreground hover:bg-accent hover:text-accent-foreground'
                } ${opt.disabled ? 'opacity-40 cursor-not-allowed' : ''}`}
              >
                <span>{opt.label}</span>
                {opt.active && (
                  <Check className="w-3 h-3 shrink-0" />
                )}
              </button>
            ))}
          </div>
        )}
      </div>
    )
  }

  const isActive = item.type === 'button' && Boolean(item.active)
  const navClass = (active: boolean) => cn(
    buttonVariants({ variant: active ? 'default' : 'ghost', size: showLabel ? 'default' : 'icon' }),
    'relative [&_svg]:size-5',
    showLabel ? 'w-full justify-start' : 'w-10 h-10',
  )

  const badge = 'badge' in item && item.badge != null ? (
    <span className="absolute -top-0.5 -right-0.5 min-w-[1.1rem] h-[1.1rem] flex items-center justify-center text-[9px] font-bold bg-surface-2 text-foreground rounded-full px-0.5">
      {item.badge}
    </span>
  ) : null

  if (item.type === 'link') {
    return (
      <NavLink to={item.to} state={item.state} title={item.title}
        className={({ isActive: linkActive }) => navClass(linkActive)}
      >
        {showIcon && item.icon}
        {showLabel && <span className="text-sm truncate">{item.title}</span>}
        {badge}
      </NavLink>
    )
  }
  return (
    <button
      onClick={item.onClick}
      disabled={item.disabled}
      title={item.title}
      className={navClass(isActive)}
    >
      {showIcon && item.icon}
      {showLabel && <span className="text-sm truncate">{item.title}</span>}
      {badge}
    </button>
  )
}

const ADMIN_SETTINGS_LINKS = [
  { to: "/settings/cameras",    label: "Câmeras" },
  { to: "/settings/discover",   label: "Rastrear câmeras" },
  { to: "/settings/users",      label: "Usuários" },
  { to: "/settings/server",     label: "Servidor" },
  { to: "/settings/storage",    label: "Armazenamento" },
  { to: "/settings/analysis",   label: "Análise de vídeo" },
  { to: "/settings/system",     label: "Sistema" },
  { to: "/settings/appearance", label: "Aparência" },
  { to: "/settings/about",      label: "Sobre" },
]

const VIEWER_SETTINGS_LINKS = ADMIN_SETTINGS_LINKS.filter(
  l => l.to === "/settings/cameras" || l.to === "/settings/appearance" || l.to === "/settings/about"
)

export default function Sidebar({ username = "usuário" }: SidebarProps) {
  const location = useLocation()
  const bellPanelRef = useRef<HTMLDivElement>(null)
  const { open: userOpen, setOpen: setUserOpen, ref: userRef } = useDropdown()
  const { open: bellOpen, setOpen: setBellOpen, ref: bellRef } = useDropdown(bellPanelRef)
  const settingsPanelRef = useRef<HTMLDivElement>(null)
  const { open: settingsOpen, setOpen: setSettingsOpen, ref: settingsRef } = useDropdown(settingsPanelRef)
  const settingsButtonRef = useRef<HTMLButtonElement>(null)
  const [settingsPos, setSettingsPos] = useState({ top: 0, left: 0 })
  const role = getRole()
  const roleLabel = role === "admin" ? "Administrador" : "Visualizador"
  const settingsLinks = role === "admin" ? ADMIN_SETTINGS_LINKS : VIEWER_SETTINGS_LINKS
  const settingsActive = location.pathname.startsWith("/settings") || location.pathname.startsWith("/stats")

  const {
    notifications, unreadCount,
    markRead, markSelectedRead,
    remove, removeAll, removeSelected,
    browserSupported, browserPermission, browserEnabled,
    enableBrowserNotifications, disableBrowserNotifications,
  } = useNotifications()

  const { unreadCount: userUnread } = useUserNotifications()

  const bellBtnRef = useRef<HTMLButtonElement>(null)
  const [bellPos, setBellPos] = useState({ top: 0, left: 0 })
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [confirm, setConfirm] = useState<ConfirmState | null>(null)
  const navigate = useNavigate()

  function openBell() {
    if (bellBtnRef.current) {
      const r = bellBtnRef.current.getBoundingClientRect()
      setBellPos({ top: r.top, left: r.right + 8 })
    }
    setBellOpen(v => !v)
  }

  const allSelected = notifications.length > 0 && notifications.every((n) => selectedIds.has(n.id))
  const someSelected = selectedIds.size > 0

  function toggleAll() {
    if (allSelected) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(notifications.map((n) => n.id)))
    }
  }

  function toggleOne(id: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) { next.delete(id) } else { next.add(id) }
      return next
    })
  }

  function targetIds() { return [...selectedIds] }
  function targetLabel() { return `${selectedIds.size} notificação(ões) selecionada(s)` }

  const selectedNotifications = notifications.filter((n) => selectedIds.has(n.id))
  const canMarkRead = selectedNotifications.some((n) => !n.read)

  function ask(state: ConfirmState) {
    setConfirm(state)
  }

  function handleConfirm() {
    confirm?.action()
    setConfirm(null)
  }

  function logout() {
    removeAll()
    clearToken()
    navigate("/login")
  }

  const items = useSidebarItems()
  const { sidebar: sidebarMode } = useDisplayMode()
  const setDisplayMode = useSetDisplayMode()
  const showIcon = sidebarMode !== 'text-only'
  const showLabel = sidebarMode !== 'icons-only'
  const collapsed = sidebarMode === 'icons-only'
  const sidebarWidth = sidebarMode === 'icons-only' ? 'w-14' : sidebarMode === 'icons-text' ? 'w-48' : 'w-40'
  const itemsAlign = showLabel ? 'items-stretch px-2' : 'items-center'
  const btnBase = showLabel
    ? 'relative flex items-center gap-2 w-full px-3 h-9 rounded-lg transition-colors'
    : 'relative flex items-center justify-center w-10 h-10 rounded-lg transition-colors'

  const navLinkClass = (active: boolean) => cn(
    buttonVariants({ variant: active ? 'default' : 'ghost', size: showLabel ? 'default' : 'icon' }),
    'relative [&_svg]:size-5',
    showLabel ? 'w-full justify-start' : 'w-10 h-10',
  )
  // Itens da nav baseados em <Button> (Eventos/sino, Configurações): mesmo tamanho
  // e padding dos NavLinks para alinhar (o Button aplica buttonVariants via props).
  const navBtnSize = showLabel ? 'default' : 'icon'
  const navBtnExtra = cn('relative [&_svg]:size-5 shadow-none', showLabel ? 'w-full justify-start' : 'w-10 h-10')

  function renderNavLink({ id, to, label, icon }: { id: string; to: string; label: string; icon: React.ReactNode }) {
    return (
      <NavLink
        key={id}
        id={id}
        to={to}
        end={to === '/'}
        title={label}
        className={({ isActive }) => navLinkClass(isActive)}
      >
        {showIcon && icon}
        {showLabel && <span className="text-sm truncate">{label}</span>}
      </NavLink>
    )
  }

  return (
    <aside className={`${sidebarWidth} flex-none flex flex-col bg-surface border-r border-border`}>
      {/* Logo */}
      <Link
        to="/"
        id="sidebar-logo"
        className={`flex items-center ${showLabel ? 'gap-2 px-4' : 'justify-center'} h-14 hover:opacity-80 transition-opacity border-b border-border flex-none`}
        title="os-camera"
      >
        <h1 className="sr-only">os-camera</h1>
        {showIcon && <CameraLogo className="w-8 h-8" />}
        {showLabel && <span className="text-sm font-semibold text-foreground truncate">os-camera</span>}
      </Link>

      {/* Nav rail principal */}
      <nav id="sidebar-nav" className={`flex flex-col ${itemsAlign} gap-1 py-2`}>
        {/* Eventos — sino com painel de notificações ao vivo (1º item do nav) */}
        <div ref={bellRef} className={showLabel ? 'w-full' : undefined}>
          <Button
            id="sidebar-notifications"
            ref={bellBtnRef}
            variant="ghost"
            size={navBtnSize}
            onClick={openBell}
            className={cn(navBtnExtra, unreadCount > 0 && "text-primary animate-pulse")}
            title="Eventos"
          >
            {showIcon && <Bell />}
            {showLabel && <span className="text-sm truncate">Eventos</span>}
            {unreadCount > 0 && (
              <span className="absolute top-1 right-1 flex items-center justify-center w-4 h-4 text-[10px] font-bold bg-red-600 text-white rounded-full">
                {unreadCount > 99 ? "99+" : unreadCount}
              </span>
            )}
          </Button>

          {bellOpen && createPortal(
            <div
              id="events-panel"
              ref={bellPanelRef}
              style={{ position: 'fixed', top: bellPos.top, left: bellPos.left, zIndex: 9999 }}
              className="w-72 bg-surface border border-border rounded shadow-lg flex flex-col max-h-[80vh]">
              <EventsPanelHeader
                allSelected={allSelected}
                someSelected={someSelected}
                canMarkRead={canMarkRead}
                onToggleAll={toggleAll}
                onMarkRead={() => ask({
                  title: "Marcar como lidas",
                  message: `Marcar ${targetLabel()} como lidas?`,
                  confirmLabel: "Marcar",
                  action: () => { markSelectedRead(targetIds()); setSelectedIds(new Set()) },
                })}
                onDelete={() => ask({
                  title: "Excluir notificações",
                  message: `Excluir ${targetLabel()}? Esta ação não pode ser desfeita.`,
                  confirmLabel: "Excluir",
                  danger: true,
                  action: () => {
                    if (someSelected) { removeSelected(targetIds()) } else { removeAll() }
                    setSelectedIds(new Set())
                  },
                })}
              />

              <div className="overflow-y-auto flex-1">
                {notifications.length === 0 ? (
                  <p className="text-xs text-gray-500 text-center py-6">Nenhuma notificação</p>
                ) : (
                  notifications.map((n) => (
                    <NotificationItem
                      key={n.id}
                      n={n}
                      checked={selectedIds.has(n.id)}
                      onToggle={() => toggleOne(n.id)}
                      onClick={() => {
                        markRead(n.id)
                        setBellOpen(false)
                        navigate(`/cameras/${n.cameraId}`, { state: { eventTime: n.time } })
                      }}
                      onRemove={() => ask({
                        title: "Excluir notificação",
                        message: "Excluir esta notificação?",
                        confirmLabel: "Excluir",
                        danger: true,
                        action: () => {
                          remove(n.id)
                          setSelectedIds((prev) => {
                            const next = new Set(prev)
                            next.delete(n.id)
                            return next
                          })
                        },
                      })}
                    />
                  ))
                )}
              </div>

              {browserSupported && (
                <div className="border-t border-border px-3 py-2 flex items-center justify-between">
                  <span className="text-xs text-gray-400">Alertas do sistema</span>
                  {browserPermission === "denied" ? (
                    <button onClick={enableBrowserNotifications} className="text-xs text-red-400 hover:text-red-300 transition-colors cursor-pointer" title="Permissão negada — tentar">
                      Permissão negada
                    </button>
                  ) : browserEnabled ? (
                    <button onClick={disableBrowserNotifications} className="text-xs text-blue-400 hover:text-blue-300 transition-colors">Desativar</button>
                  ) : (
                    <button onClick={enableBrowserNotifications} className="text-xs text-gray-400 hover:text-white transition-colors">Ativar</button>
                  )}
                </div>
              )}
            </div>
          , document.body)}
        </div>

        {NAV_LINKS.map(renderNavLink)}

        {/* Configurações — flyout com as seções de config */}
        <div ref={settingsRef} className={showLabel ? 'w-full' : undefined}>
          <Button
            id="sidebar-settings"
            ref={settingsButtonRef}
            variant={settingsActive ? 'default' : 'ghost'}
            onClick={() => {
              if (settingsButtonRef.current) {
                const r = settingsButtonRef.current.getBoundingClientRect()
                setSettingsPos({ top: r.top, left: r.right + 8 })
              }
              setSettingsOpen(v => !v)
            }}
            title="Configurações"
            size={navBtnSize}
            className={navBtnExtra}
          >
            {showIcon && <Settings />}
            {showLabel && <span className="text-sm truncate">Configurações</span>}
          </Button>
        </div>
        {settingsOpen && createPortal(
          <div
            ref={settingsPanelRef}
            style={{ position: 'fixed', top: settingsPos.top, left: settingsPos.left, zIndex: 9999 }}
            className="w-48 bg-surface border border-border rounded shadow-lg"
          >
            <div className="px-3 py-2 text-xs text-gray-500 border-b border-border font-medium">Configurações</div>
            {settingsLinks.map(({ to, label }) => (
              <Fragment key={to}>
                {to === '/settings/about' && <ThemeModeNav onSelect={() => setSettingsOpen(false)} />}
                {to === '/settings/about' && (
                  <Link
                    to="/stats"
                    id="settings-stats"
                    onClick={() => setSettingsOpen(false)}
                    className={`block px-3 py-2 text-sm transition-colors ${
                      location.pathname.startsWith('/stats')
                        ? 'bg-accent text-accent-foreground'
                        : 'text-foreground hover:bg-accent hover:text-accent-foreground'
                    }`}
                  >
                    Estatísticas
                  </Link>
                )}
                <Link
                  to={to}
                  onClick={() => setSettingsOpen(false)}
                  className={`block px-3 py-2 text-sm transition-colors ${
                    location.pathname.startsWith(to)
                      ? 'bg-accent text-accent-foreground'
                      : 'text-foreground hover:bg-accent hover:text-accent-foreground'
                  }`}
                >
                  {label}
                </Link>
              </Fragment>
            ))}
          </div>,
          document.body
        )}
      </nav>

      {/* Injected camera items */}
      {items.length > 0 && (
        <div className={`flex flex-col ${itemsAlign} gap-1 py-2 border-t border-border flex-none`}>
          {items.map(item => <SidebarInjectedItem key={item.id} item={item} displayMode={sidebarMode} />)}
        </div>
      )}

      {/* Spacer */}
      <div className="flex-1" />

      {/* Bottom: Recolher + User */}
      <div id="sidebar-bottom" className={`flex flex-col ${itemsAlign} gap-1 pb-3 border-t border-border pt-2 flex-none`}>

        {/* Recolher menu */}
        <button
          id="sidebar-collapse"
          onClick={() => setDisplayMode('sidebar', collapsed ? 'icons-text' : 'icons-only')}
          title={collapsed ? 'Expandir menu' : 'Recolher menu'}
          className={cn(btnBase, 'text-muted-foreground hover:bg-accent hover:text-accent-foreground [&_svg]:size-5')}
        >
          <ChevronLeft className={collapsed ? 'rotate-180' : ''} />
          {showLabel && <span className="text-sm truncate">Recolher menu</span>}
        </button>

        {/* User */}
        <div className={`relative ${showLabel ? 'w-full' : ''}`} ref={userRef}>
          <Button
            id="sidebar-user"
            variant="ghost"
            onClick={() => setUserOpen((v) => !v)}
            className={cn(
              btnBase,
              'shadow-none [&_svg]:size-6',
              showLabel && 'h-auto py-1.5',
            )}
            title={username}
          >
            {showIcon && <CircleUser />}
            {showLabel && (
              <span className="flex flex-col items-start min-w-0 leading-tight">
                <span className="text-sm text-foreground truncate max-w-full">{username}</span>
                <span className="text-xs text-muted-foreground truncate max-w-full">{roleLabel}</span>
              </span>
            )}
            {userUnread > 0 && (
              <span className="absolute top-1 left-6 min-w-[16px] h-4 px-1 rounded-full bg-primary text-primary-foreground text-[10px] font-semibold flex items-center justify-center">
                {userUnread > 99 ? '99+' : userUnread}
              </span>
            )}
          </Button>

          {userOpen && (
            <div className="absolute left-full bottom-0 ml-2 w-44 bg-surface border border-border rounded shadow-lg z-50">
              <div className="px-4 py-2 text-xs text-gray-500 border-b border-border truncate">{username} · {roleLabel}</div>
              <Link
                to="/notifications"
                onClick={() => setUserOpen(false)}
                className="flex items-center justify-between w-full text-left px-4 py-2 text-sm text-foreground hover:bg-accent hover:text-accent-foreground"
              >
                <span>Notificações</span>
                {userUnread > 0 && (
                  <span className="min-w-[16px] h-4 px-1 rounded-full bg-primary text-primary-foreground text-[10px] font-semibold flex items-center justify-center">
                    {userUnread > 99 ? '99+' : userUnread}
                  </span>
                )}
              </Link>
              <Link
                to="/change-password"
                state={{ from: location.pathname }}
                onClick={() => setUserOpen(false)}
                className="block w-full text-left px-4 py-2 text-sm text-foreground hover:bg-accent hover:text-accent-foreground"
              >
                Alterar senha
              </Link>
              <div className="border-t border-border" />
              <button
                onClick={logout}
                className="block w-full text-left px-4 py-2 text-sm text-foreground hover:bg-accent hover:text-accent-foreground"
              >
                Sair
              </button>
            </div>
          )}
        </div>
      </div>

      <ConfirmDialog
        open={confirm !== null}
        title={confirm?.title ?? ""}
        message={confirm?.message ?? ""}
        confirmLabel={confirm?.confirmLabel}
        danger={confirm?.danger}
        onConfirm={handleConfirm}
        onCancel={() => setConfirm(null)}
      />
    </aside>
  )
}
