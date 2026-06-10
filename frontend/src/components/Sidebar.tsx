import { useState, useRef, useEffect } from "react"
import { createPortal } from "react-dom"
import { useNavigate, useLocation, Link, NavLink } from "react-router-dom"
import { clearToken, getRole, authHeaders, onUnauthorized } from "../auth"
import { useNotifications } from "../contexts/NotificationContext"
import { useUserNotifications } from "../contexts/UserNotificationContext"
import { useSidebarItems, type SidebarItem, type SidebarDropdownOption } from "../contexts/SidebarContext"
import { useDisplayMode } from "../contexts/DisplayModeContext"
import { formatDistanceToNow } from "date-fns"
import { ptBR } from "date-fns/locale"
import type { Notification } from "../contexts/NotificationContext"
import ConfirmDialog from "./ConfirmDialog"
import EventsPanelHeader from "./EventsPanelHeader"
import { Bell, X, Check, BarChart2, Settings, CircleUser, CameraLogo, Cctv } from "./Icons"

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
      className={`flex items-start gap-2 px-3 py-2 hover:bg-gray-750 transition-colors ${
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
      <button
        onClick={(e) => { e.stopPropagation(); onRemove() }}
        className="text-gray-500 hover:text-gray-300 flex-shrink-0 mt-0.5"
        title="Excluir"
      >
        <X className="w-3.5 h-3.5" />
      </button>
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
    return <div className={showLabel ? 'w-full border-t border-gray-700 my-1' : 'w-8 border-t border-gray-700 my-1'} />
  }

  if (item.type === 'dropdown') {
    const isActive = item.active || dropOpen
    return (
      <div className={showLabel ? 'relative w-full' : 'relative'} ref={dropRef}>
        <button
          onClick={() => { if (!item.disabled) setDropOpen(v => !v) }}
          disabled={item.disabled}
          title={item.title}
          className={`${base} ${isActive ? 'bg-blue-600 text-white' : 'text-gray-400 hover:bg-gray-800 hover:text-white'} ${item.disabled ? 'opacity-40 cursor-not-allowed' : ''}`}
        >
          {showIcon && item.icon}
          {showLabel && <span className="text-sm truncate">{item.title}</span>}
        </button>
        {dropOpen && (
          <div className="absolute left-full top-0 ml-2 bg-gray-800 border border-gray-700 rounded-lg shadow-xl z-50 py-1 min-w-[72px]">
            {item.options.map((opt: SidebarDropdownOption, i: number) => (
              <button
                key={i}
                onClick={() => { opt.onClick(); setDropOpen(false) }}
                disabled={opt.disabled}
                className={`flex items-center justify-between gap-2 w-full px-3 py-1.5 text-sm text-left transition-colors ${
                  opt.active
                    ? 'text-blue-400 font-semibold'
                    : 'text-gray-300 hover:bg-gray-700 hover:text-white'
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

  const isActive = item.type === 'button' && item.active
  const activeClass = isActive ? "bg-blue-600 text-white" : "text-gray-400 hover:bg-gray-800 hover:text-white"
  const disabledClass = (item.type === 'button' && item.disabled) ? "opacity-40 cursor-not-allowed" : ""

  const badge = 'badge' in item && item.badge != null ? (
    <span className="absolute -top-0.5 -right-0.5 min-w-[1.1rem] h-[1.1rem] flex items-center justify-center text-[9px] font-bold bg-gray-700 text-gray-200 rounded-full px-0.5">
      {item.badge}
    </span>
  ) : null

  if (item.type === 'link') {
    return (
      <NavLink to={item.to} state={item.state} title={item.title}
        className={({ isActive: linkActive }) =>
          `${base} ${linkActive ? 'bg-blue-600 text-white' : 'text-gray-400 hover:bg-gray-800 hover:text-white'}`
        }
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
      className={`${base} ${activeClass} ${disabledClass}`}
    >
      {showIcon && item.icon}
      {showLabel && <span className="text-sm truncate">{item.title}</span>}
      {badge}
    </button>
  )
}

function NavIcon({ to, title, id, label, children, end, displayMode }: {
  to: string; title: string; id?: string; label?: string
  children: React.ReactNode; end?: boolean
  displayMode: 'icons-only' | 'icons-text' | 'text-only'
}) {
  const showIcon = displayMode !== 'text-only'
  const showLabel = displayMode !== 'icons-only'
  return (
    <NavLink
      to={to}
      end={end}
      id={id}
      title={title}
      className={({ isActive }) =>
        `flex items-center gap-2 rounded-lg transition-colors ${
          showLabel ? 'w-full px-3 h-9' : 'justify-center w-10 h-10'
        } ${isActive ? "bg-blue-600 text-white" : "text-gray-400 hover:bg-gray-800 hover:text-white"}`
      }
    >
      {showIcon && children}
      {showLabel && label && <span className="text-sm truncate">{label}</span>}
    </NavLink>
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
  const [settingsPos, setSettingsPos] = useState({ bottom: 0, left: 0 })
  const settingsLinks = getRole() === "admin" ? ADMIN_SETTINGS_LINKS : VIEWER_SETTINGS_LINKS
  const settingsActive = location.pathname.startsWith("/settings")

  const camerasPanelRef = useRef<HTMLDivElement>(null)
  const { open: camerasOpen, setOpen: setCamerasOpen, ref: camerasRef } = useDropdown(camerasPanelRef)
  const camerasButtonRef = useRef<HTMLButtonElement>(null)
  const [camerasPos, setCamerasPos] = useState({ bottom: 0, left: 0 })
  const [cameraList, setCameraList] = useState<{ id: string; name: string }[]>([])
  const camerasActive = location.pathname.startsWith("/camera")

  function openCameras() {
    if (camerasButtonRef.current) {
      const r = camerasButtonRef.current.getBoundingClientRect()
      setCamerasPos({ bottom: window.innerHeight - r.bottom, left: r.right + 8 })
    }
    if (!camerasOpen) {
      fetch('/api/cameras', { headers: authHeaders() })
        .then(res => { if (res.status === 401) { onUnauthorized(); return null } return res.json() })
        .then(data => { if (Array.isArray(data)) setCameraList(data) })
    }
    setCamerasOpen(v => !v)
  }

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
  const showIcon = sidebarMode !== 'text-only'
  const showLabel = sidebarMode !== 'icons-only'
  const sidebarWidth = sidebarMode === 'icons-only' ? 'w-14' : sidebarMode === 'icons-text' ? 'w-48' : 'w-40'
  const itemsAlign = showLabel ? 'items-stretch px-2' : 'items-center'
  const btnBase = showLabel
    ? 'relative flex items-center gap-2 w-full px-3 h-9 rounded-lg transition-colors'
    : 'relative flex items-center justify-center w-10 h-10 rounded-lg transition-colors'

  return (
    <aside className={`${sidebarWidth} flex-none flex flex-col bg-gray-900 border-r border-gray-800`}>
      {/* Logo */}
      <Link
        to="/"
        id="sidebar-logo"
        className={`flex items-center ${showLabel ? 'gap-2 px-4' : 'justify-center'} h-14 hover:opacity-80 transition-opacity border-b border-gray-800 flex-none`}
        title="os-camera"
      >
        <h1 className="sr-only">os-camera</h1>
        {showIcon && <CameraLogo className="w-8 h-8" />}
        {showLabel && <span className="text-sm font-semibold text-gray-200 truncate">os-camera</span>}
      </Link>


      {/* Bell — logo abaixo da logo */}
      <div className={`flex flex-col ${itemsAlign} py-1 border-b border-gray-800 flex-none`}>
        <div ref={bellRef} className={showLabel ? 'w-full' : undefined}>
          <button
            id="sidebar-notifications"
            ref={bellBtnRef}
            onClick={openBell}
            className={`${btnBase} ${unreadCount > 0 ? "text-white animate-pulse" : "text-gray-400 hover:bg-gray-800 hover:text-white"}`}
            title="Notificações"
          >
            {showIcon && <Bell className="w-5 h-5" />}
            {showLabel && <span className="text-sm truncate">Notificações</span>}
            {unreadCount > 0 && (
              <span className="absolute top-1 right-1 flex items-center justify-center w-4 h-4 text-[10px] font-bold bg-red-600 text-white rounded-full">
                {unreadCount > 99 ? "99+" : unreadCount}
              </span>
            )}
          </button>

          {bellOpen && createPortal(
            <div
              id="events-panel"
              ref={bellPanelRef}
              style={{ position: 'fixed', top: bellPos.top, left: bellPos.left, zIndex: 9999 }}
              className="w-72 bg-gray-800 border border-gray-700 rounded shadow-lg flex flex-col max-h-[80vh]">
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
                <div className="border-t border-gray-700 px-3 py-2 flex items-center justify-between">
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
      </div>

      {/* Injected camera items */}
      {items.length > 0 && (
        <div className={`flex flex-col ${itemsAlign} gap-1 py-2 border-b border-gray-800 flex-none`}>
          {items.map(item => <SidebarInjectedItem key={item.id} item={item} displayMode={sidebarMode} />)}
        </div>
      )}

      {/* Spacer */}
      <div className="flex-1" />

      {/* Bottom: Stats + Settings + User */}
      <div className={`flex flex-col ${itemsAlign} gap-1 pb-3 flex-none`}>

        {/* Stats */}
        <NavIcon id="sidebar-stats" to="/stats" title="Estatísticas" label="Estatísticas" displayMode={sidebarMode}>
          <BarChart2 className="w-5 h-5" />
        </NavIcon>

        {/* Cameras */}
        <div ref={camerasRef} className={showLabel ? 'w-full' : undefined}>
          <button
            id="sidebar-cameras"
            ref={camerasButtonRef}
            onClick={openCameras}
            title="Câmeras"
            className={`${btnBase} ${
              camerasActive || camerasOpen
                ? 'bg-blue-600 text-white'
                : 'text-gray-400 hover:bg-gray-800 hover:text-white'
            }`}
          >
            {showIcon && <Cctv className="w-5 h-5" />}
            {showLabel && <span className="text-sm truncate">Câmeras</span>}
          </button>
        </div>
        {camerasOpen && createPortal(
          <div
            ref={camerasPanelRef}
            style={{ position: 'fixed', bottom: camerasPos.bottom, left: camerasPos.left, zIndex: 9999 }}
            className="w-48 bg-gray-800 border border-gray-700 rounded shadow-lg"
          >
            <div className="px-3 py-2 text-xs text-gray-500 border-b border-gray-700 font-medium">Câmeras</div>
            {cameraList.length === 0 ? (
              <div className="px-3 py-2 text-sm text-gray-500">Nenhuma câmera</div>
            ) : cameraList.map(cam => (
              <button
                key={cam.id}
                onClick={() => {
                  setCamerasOpen(false)
                  navigate(`/camera/live/${cam.id}`, { replace: true, state: { goLive: Date.now() } })
                }}
                className={`block w-full text-left px-3 py-2 text-sm transition-colors truncate ${
                  location.pathname === `/camera/live/${cam.id}`
                    ? 'bg-gray-700 text-white'
                    : 'text-gray-300 hover:bg-gray-700 hover:text-white'
                }`}
              >
                {cam.name}
              </button>
            ))}
          </div>,
          document.body
        )}

        {/* Settings */}
        <div ref={settingsRef} className={showLabel ? 'w-full' : undefined}>
          <button
            id="sidebar-settings"
            ref={settingsButtonRef}
            onClick={() => {
              if (settingsButtonRef.current) {
                const r = settingsButtonRef.current.getBoundingClientRect()
                setSettingsPos({ bottom: window.innerHeight - r.bottom, left: r.right + 8 })
              }
              setSettingsOpen(v => !v)
            }}
            title="Configurações"
            className={`${btnBase} ${
              settingsActive || settingsOpen
                ? 'bg-blue-600 text-white'
                : 'text-gray-400 hover:bg-gray-800 hover:text-white'
            }`}
          >
            {showIcon && <Settings className="w-5 h-5" />}
            {showLabel && <span className="text-sm truncate">Configurações</span>}
          </button>
        </div>
        {settingsOpen && createPortal(
          <div
            ref={settingsPanelRef}
            style={{ position: 'fixed', bottom: settingsPos.bottom, left: settingsPos.left, zIndex: 9999 }}
            className="w-48 bg-gray-800 border border-gray-700 rounded shadow-lg"
          >
            <div className="px-3 py-2 text-xs text-gray-500 border-b border-gray-700 font-medium">Configurações</div>
            {settingsLinks.map(({ to, label }) => (
              <Link
                key={to}
                to={to}
                onClick={() => setSettingsOpen(false)}
                className={`block px-3 py-2 text-sm transition-colors ${
                  location.pathname.startsWith(to)
                    ? 'bg-gray-700 text-white'
                    : 'text-gray-300 hover:bg-gray-700 hover:text-white'
                }`}
              >
                {label}
              </Link>
            ))}
          </div>,
          document.body
        )}

        {/* User */}
        <div className={`relative ${showLabel ? 'w-full' : ''}`} ref={userRef}>
          <button
            id="sidebar-user"
            onClick={() => setUserOpen((v) => !v)}
            className={`relative ${btnBase} text-gray-400 hover:bg-gray-800 hover:text-white`}
            title={username}
          >
            {showIcon && <CircleUser className="w-6 h-6" />}
            {showLabel && <span className="text-sm truncate">{username}</span>}
            {userUnread > 0 && (
              <span className="absolute top-1 left-6 min-w-[16px] h-4 px-1 rounded-full bg-blue-600 text-white text-[10px] font-semibold flex items-center justify-center">
                {userUnread > 99 ? '99+' : userUnread}
              </span>
            )}
          </button>

          {userOpen && (
            <div className="absolute left-full bottom-0 ml-2 w-44 bg-gray-800 border border-gray-700 rounded shadow-lg z-50">
              <div className="px-4 py-2 text-xs text-gray-500 border-b border-gray-700 truncate">{username}</div>
              <Link
                to="/notifications"
                onClick={() => setUserOpen(false)}
                className="flex items-center justify-between w-full text-left px-4 py-2 text-sm text-gray-300 hover:bg-gray-700 hover:text-white"
              >
                <span>Notificações</span>
                {userUnread > 0 && (
                  <span className="min-w-[16px] h-4 px-1 rounded-full bg-blue-600 text-white text-[10px] font-semibold flex items-center justify-center">
                    {userUnread > 99 ? '99+' : userUnread}
                  </span>
                )}
              </Link>
              <Link
                to="/change-password"
                onClick={() => setUserOpen(false)}
                className="block w-full text-left px-4 py-2 text-sm text-gray-300 hover:bg-gray-700 hover:text-white"
              >
                Alterar senha
              </Link>
              <div className="border-t border-gray-700" />
              <button
                onClick={logout}
                className="block w-full text-left px-4 py-2 text-sm text-gray-300 hover:bg-gray-700 hover:text-white"
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
