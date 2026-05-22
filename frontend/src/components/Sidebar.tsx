import { useState, useRef, useEffect } from "react"
import { useNavigate, Link, NavLink } from "react-router-dom"
import { clearToken } from "../auth"
import { useNotifications } from "../contexts/NotificationContext"
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

function useDropdown() {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node))
        setOpen(false)
    }
    document.addEventListener("mousedown", handleClick)
    return () => document.removeEventListener("mousedown", handleClick)
  }, [])
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

function NavIcon({ to, title, children, end }: { to: string; title: string; children: React.ReactNode; end?: boolean }) {
  return (
    <NavLink
      to={to}
      end={end}
      title={title}
      className={({ isActive }) =>
        `flex items-center justify-center w-10 h-10 rounded-lg transition-colors ${
          isActive ? "bg-gray-800 text-white" : "text-gray-400 hover:bg-gray-800 hover:text-white"
        }`
      }
    >
      {children}
    </NavLink>
  )
}

export default function Sidebar({ username = "usuário" }: SidebarProps) {
  const { open: userOpen, setOpen: setUserOpen, ref: userRef } = useDropdown()
  const { open: bellOpen, setOpen: setBellOpen, ref: bellRef } = useDropdown()
  const { open: kebabOpen, setOpen: setKebabOpen, ref: kebabRef } = useDropdown()

  const {
    notifications, unreadCount,
    markRead, markSelectedRead, markAllUnread,
    remove, removeAll, removeSelected,
    browserSupported, browserPermission, browserEnabled,
    enableBrowserNotifications, disableBrowserNotifications,
  } = useNotifications()

  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [confirm, setConfirm] = useState<ConfirmState | null>(null)
  const navigate = useNavigate()

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

      {/* Nav */}
      <nav className="flex flex-col items-center gap-1 py-3 flex-none">
        <NavIcon to="/settings/stats" title="Configurações">
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
              d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
            />
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
        </NavIcon>
      </nav>

      {/* Spacer */}
      <div className="flex-1" />

      {/* Bottom: Bell + User */}
      <div className="flex flex-col items-center gap-1 pb-3 flex-none">

        {/* Bell */}
        <div className="relative" ref={bellRef}>
          <button
            onClick={() => setBellOpen((v) => !v)}
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

          {bellOpen && (
            <div className="absolute left-full bottom-0 ml-2 w-72 bg-gray-800 border border-gray-700 rounded shadow-lg z-50 flex flex-col max-h-[80vh]">
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
          )}
        </div>

        {/* User */}
        <div className="relative" ref={userRef}>
          <button
            onClick={() => setUserOpen((v) => !v)}
            className="flex items-center justify-center w-10 h-10 rounded-lg text-gray-400 hover:bg-gray-800 hover:text-white transition-colors"
            title={username}
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
              />
            </svg>
          </button>

          {userOpen && (
            <div className="absolute left-full bottom-0 ml-2 w-44 bg-gray-800 border border-gray-700 rounded shadow-lg z-50">
              <div className="px-4 py-2 text-xs text-gray-500 border-b border-gray-700 truncate">{username}</div>
              <Link
                to="/settings/stats"
                onClick={() => setUserOpen(false)}
                className="block w-full text-left px-4 py-2 text-sm text-gray-300 hover:bg-gray-700 hover:text-white"
              >
                Configurações
              </Link>
              <div className="border-t border-gray-700" />
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
