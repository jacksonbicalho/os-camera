import { useState, useRef, useEffect } from "react"
import { createPortal } from "react-dom"
import { useNavigate, Link, NavLink } from "react-router-dom"
import { clearToken } from "../auth"
import { useNotifications } from "../contexts/NotificationContext"
import { useSidebarItems, type SidebarItem, type SidebarDropdownOption } from "../contexts/SidebarContext"
import { formatDistanceToNow } from "date-fns"
import { ptBR } from "date-fns/locale"
import type { Notification } from "../contexts/NotificationContext"
import ConfirmDialog from "./ConfirmDialog"

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
        <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>
  )
}

function SidebarInjectedItem({ item }: { item: SidebarItem }) {
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

  if (item.type === 'separator') {
    return <div className="w-8 border-t border-gray-700 my-1" />
  }

  const base = `relative flex items-center justify-center w-10 h-10 rounded-lg transition-colors`

  if (item.type === 'dropdown') {
    const isActive = item.active || dropOpen
    return (
      <div className="relative" ref={dropRef}>
        <button
          onClick={() => { if (!item.disabled) setDropOpen(v => !v) }}
          disabled={item.disabled}
          title={item.title}
          className={`${base} ${isActive ? 'bg-blue-600 text-white' : 'text-gray-400 hover:bg-gray-800 hover:text-white'} ${item.disabled ? 'opacity-40 cursor-not-allowed' : ''}`}
        >
          {item.icon}
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
                  <svg className="w-3 h-3 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={3} d="M5 13l4 4L19 7" />
                  </svg>
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
        {item.icon}
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
      {item.icon}
      {badge}
    </button>
  )
}

function NavIcon({ to, title, children, end }: { to: string; title: string; children: React.ReactNode; end?: boolean }) {
  return (
    <NavLink
      to={to}
      end={end}
      title={title}
      className={({ isActive }) =>
        `flex items-center justify-center w-10 h-10 rounded-lg transition-colors ${
          isActive ? "bg-blue-600 text-white" : "text-gray-400 hover:bg-gray-800 hover:text-white"
        }`
      }
    >
      {children}
    </NavLink>
  )
}

export default function Sidebar({ username = "usuário" }: SidebarProps) {
  const bellPanelRef = useRef<HTMLDivElement>(null)
  const { open: userOpen, setOpen: setUserOpen, ref: userRef } = useDropdown()
  const { open: bellOpen, setOpen: setBellOpen, ref: bellRef } = useDropdown(bellPanelRef)
  const { open: kebabOpen, setOpen: setKebabOpen, ref: kebabRef } = useDropdown()

  const {
    notifications, unreadCount,
    markRead, markSelectedRead, markAllUnread,
    remove, removeAll, removeSelected,
    browserSupported, browserPermission, browserEnabled,
    enableBrowserNotifications, disableBrowserNotifications,
  } = useNotifications()

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

  useEffect(() => {
    if (!bellOpen) setKebabOpen(false)
  }, [bellOpen, setKebabOpen])

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
  const canMarkUnread = selectedNotifications.some((n) => n.read)

  function ask(state: ConfirmState) {
    setKebabOpen(false)
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
  return (
    <aside className="w-14 flex-none flex flex-col bg-gray-900 border-r border-gray-800">
      {/* Logo */}
      <Link
        to="/"
        className="flex items-center justify-center h-14 text-white hover:text-gray-300 transition-colors border-b border-gray-800 flex-none"
        title="Camera"
      >
        <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
            d="M15 10l4.553-2.276A1 1 0 0121 8.723v6.554a1 1 0 01-1.447.894L15 14M3 8a2 2 0 012-2h10a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V8z"
          />
        </svg>
      </Link>


      {/* Bell — logo abaixo da logo */}
      <div className="flex flex-col items-center py-1 border-b border-gray-800 flex-none">
        <div ref={bellRef}>
          <button
            ref={bellBtnRef}
            onClick={openBell}
            className={`relative flex items-center justify-center w-10 h-10 rounded-lg transition-colors ${
              unreadCount > 0 ? "text-white animate-pulse" : "text-gray-400 hover:bg-gray-800 hover:text-white"
            }`}
            title="Notificações"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"
              />
            </svg>
            {unreadCount > 0 && (
              <span className="absolute top-1 right-1 flex items-center justify-center w-4 h-4 text-[10px] font-bold bg-red-600 text-white rounded-full">
                {unreadCount > 99 ? "99+" : unreadCount}
              </span>
            )}
          </button>

          {bellOpen && createPortal(
            <div
              ref={bellPanelRef}
              style={{ position: 'fixed', top: bellPos.top, left: bellPos.left, zIndex: 9999 }}
              className="w-72 bg-gray-800 border border-gray-700 rounded shadow-lg flex flex-col max-h-[80vh]">
              <div className="px-3 pt-2.5 pb-0">
                <span className="text-xs font-semibold text-gray-300">Notificações</span>
              </div>

              <div className="flex items-center justify-between px-3 py-1.5 border-b border-gray-700">
                <label className="flex items-center gap-1.5 cursor-pointer select-none">
                  <input
                    type="checkbox"
                    checked={allSelected}
                    ref={(el) => { if (el) el.indeterminate = someSelected && !allSelected }}
                    onChange={toggleAll}
                    className="w-3 h-3 accent-blue-500 cursor-pointer"
                  />
                  <span className="text-xs text-gray-400">Selecionar todos</span>
                </label>

                <div className="relative" ref={kebabRef}>
                  <button
                    onClick={() => setKebabOpen((v) => !v)}
                    disabled={!someSelected}
                    className="flex items-center gap-0.5 text-gray-400 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
                    title="Ações"
                  >
                    <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                      <circle cx="12" cy="5" r="1.5" />
                      <circle cx="12" cy="12" r="1.5" />
                      <circle cx="12" cy="19" r="1.5" />
                    </svg>
                  </button>

                  {kebabOpen && (
                    <div className="absolute right-0 top-full mt-1 bg-gray-900 border border-gray-700 rounded shadow-lg z-20 whitespace-nowrap min-w-[180px]">
                      {canMarkRead && (
                        <button
                          onClick={() => ask({
                            title: "Marcar como lidas",
                            message: `Marcar ${targetLabel()} como lidas?`,
                            confirmLabel: "Marcar",
                            action: () => { markSelectedRead(targetIds()); setSelectedIds(new Set()) },
                          })}
                          className="flex items-center gap-2 w-full text-left px-3 py-2 text-xs text-gray-300 hover:bg-gray-700"
                        >
                          <svg className="w-3.5 h-3.5 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                          </svg>
                          Marcar como lidas
                        </button>
                      )}
                      {canMarkUnread && (
                        <button
                          onClick={() => ask({
                            title: "Marcar como não lidas",
                            message: `Marcar ${targetLabel()} como não lidas?`,
                            confirmLabel: "Marcar",
                            action: () => { markAllUnread(targetIds()); setSelectedIds(new Set()) },
                          })}
                          className="flex items-center gap-2 w-full text-left px-3 py-2 text-xs text-gray-300 hover:bg-gray-700"
                        >
                          <svg className="w-3.5 h-3.5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                          </svg>
                          Marcar como não lidas
                        </button>
                      )}
                      {(canMarkRead || canMarkUnread) && <div className="border-t border-gray-700 my-1" />}
                      <button
                        onClick={() => ask({
                          title: "Excluir notificações",
                          message: `Excluir ${targetLabel()}? Esta ação não pode ser desfeita.`,
                          confirmLabel: "Excluir",
                          danger: true,
                          action: () => {
                            if (someSelected) { removeSelected(targetIds()) } else { removeAll() }
                            setSelectedIds(new Set())
                          },
                        })}
                        className="flex items-center gap-2 w-full text-left px-3 py-2 text-xs text-red-400 hover:bg-gray-700"
                      >
                        <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                        Excluir
                      </button>
                    </div>
                  )}
                </div>
              </div>

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
        <div className="flex flex-col items-center gap-0.5 py-2 border-b border-gray-800 flex-none">
          {items.map(item => <SidebarInjectedItem key={item.id} item={item} />)}
        </div>
      )}

      {/* Spacer */}
      <div className="flex-1" />

      {/* Bottom: Stats + Settings + User */}
      <div className="flex flex-col items-center gap-1 pb-3 flex-none">

        {/* Stats */}
        <NavIcon to="/stats" title="Estatísticas">
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
              d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"
            />
          </svg>
        </NavIcon>

        {/* Settings */}
        <NavLink
          to="/settings/cameras"
          title="Configurações"
          className={({ isActive }) =>
            `relative flex items-center justify-center w-10 h-10 rounded-lg transition-colors ${
              isActive ? 'bg-blue-600 text-white' : 'text-gray-400 hover:bg-gray-800 hover:text-white'
            }`
          }
        >
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
              d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
            />
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
        </NavLink>

        {/* User */}
        <div className="relative" ref={userRef}>
          <button
            onClick={() => setUserOpen((v) => !v)}
            className="flex items-center justify-center w-10 h-10 rounded-lg text-gray-400 hover:bg-gray-800 hover:text-white transition-colors"
            title={username}
          >
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6">
              <path strokeLinecap="round" strokeLinejoin="round" d="M17.982 18.725A7.488 7.488 0 0 0 12 15.75a7.488 7.488 0 0 0-5.982 2.975m11.963 0a9 9 0 1 0-11.963 0m11.963 0A8.966 8.966 0 0 1 12 21a8.966 8.966 0 0 1-5.982-2.275M15 9.75a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
            </svg>
          </button>

          {userOpen && (
            <div className="absolute left-full bottom-0 ml-2 w-44 bg-gray-800 border border-gray-700 rounded shadow-lg z-50">
              <div className="px-4 py-2 text-xs text-gray-500 border-b border-gray-700 truncate">{username}</div>
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
