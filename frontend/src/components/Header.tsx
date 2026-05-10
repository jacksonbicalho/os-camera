import { useState, useRef, useEffect } from "react";
import { useNavigate, NavLink, Link } from "react-router-dom";
import { clearToken } from "../auth";
import { useNotifications } from "../contexts/NotificationContext";
import { formatDistanceToNow } from "date-fns";
import { ptBR } from "date-fns/locale";
import type { Notification } from "../contexts/NotificationContext";
import ConfirmDialog from "./ConfirmDialog";

interface HeaderProps {
  username?: string;
}

interface ConfirmState {
  title: string
  message: string
  confirmLabel?: string
  danger?: boolean
  action: () => void
}

function useDropdown() {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node))
        setOpen(false);
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);
  return { open, setOpen, ref };
}

function NotificationItem({
  n,
  checked,
  onToggle,
  onClick,
  onRemove,
}: {
  n: Notification;
  checked: boolean;
  onToggle: () => void;
  onClick: () => void;
  onRemove: () => void;
}) {
  const relTime = formatDistanceToNow(new Date(n.time), {
    addSuffix: true,
    locale: ptBR,
  });

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
          {n.cameraId}
        </div>
        <div className="text-xs text-gray-400">
          score {n.score.toFixed(3)} · {relTime}
        </div>
      </div>
      <button
        onClick={(e) => {
          e.stopPropagation();
          onRemove();
        }}
        className="text-gray-500 hover:text-gray-300 flex-shrink-0 mt-0.5"
        title="Excluir"
      >
        <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>
  );
}

export default function Header({ username = "usuário" }: HeaderProps) {
  const { open: userOpen, setOpen: setUserOpen, ref: userRef } = useDropdown();
  const { open: bellOpen, setOpen: setBellOpen, ref: bellRef } = useDropdown();
  const { open: kebabOpen, setOpen: setKebabOpen, ref: kebabRef } = useDropdown();

  const {
    notifications, unreadCount,
    markRead, markSelectedRead, markAllUnread,
    remove, removeAll, removeSelected,
  } = useNotifications();

  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [confirm, setConfirm] = useState<ConfirmState | null>(null);
  const navigate = useNavigate();

  useEffect(() => {
    if (!bellOpen) setKebabOpen(false);
  }, [bellOpen, setKebabOpen]);

  const allSelected =
    notifications.length > 0 && notifications.every((n) => selectedIds.has(n.id));
  const someSelected = selectedIds.size > 0;

  function toggleAll() {
    if (allSelected) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(notifications.map((n) => n.id)));
    }
  }

  function toggleOne(id: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) { next.delete(id) } else { next.add(id) }
      return next;
    });
  }

  // IDs alvo: selecionados (se houver) ou todos
  function targetIds() {
    return someSelected ? [...selectedIds] : notifications.map((n) => n.id);
  }

  function targetLabel() {
    return someSelected
      ? `${selectedIds.size} notificação(ões) selecionada(s)`
      : `todas as ${notifications.length} notificações`;
  }

  // Opções disponíveis com base no estado das notificações alvo
  const targetNotifications = someSelected
    ? notifications.filter((n) => selectedIds.has(n.id))
    : notifications;
  const canMarkRead = targetNotifications.some((n) => !n.read);
  const canMarkUnread = targetNotifications.some((n) => n.read);

  function ask(state: ConfirmState) {
    setKebabOpen(false);
    setConfirm(state);
  }

  function handleConfirm() {
    confirm?.action();
    setConfirm(null);
  }

  function logout() {
    removeAll();
    clearToken();
    navigate("/login");
  }

  return (
    <header className="flex items-center justify-between px-6 py-3 bg-gray-900 border-b border-gray-800">
      <Link
        to="/"
        className="text-white font-semibold tracking-wide hover:text-gray-200 transition-colors"
      >
        📷 Camera
      </Link>

      <div className="flex items-center gap-4">
        <NavLink
          to="/settings/stats"
          className={({ isActive }) =>
            `text-sm transition-colors ${isActive || location.pathname.startsWith("/settings") ? "text-white" : "text-gray-400 hover:text-white"}`
          }
        >
          Configurações
        </NavLink>

        {/* Sino de notificações */}
        <div className="relative" ref={bellRef}>
          <button
            onClick={() => setBellOpen((v) => !v)}
            className={`relative transition-colors ${unreadCount > 0 ? "text-white animate-pulse" : "text-gray-400 hover:text-white"}`}
            title="Notificações"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"
              />
            </svg>
            {unreadCount > 0 && (
              <span className="absolute -top-1 -right-1 flex items-center justify-center w-4 h-4 text-[10px] font-bold bg-red-600 text-white rounded-full">
                {unreadCount > 99 ? "99+" : unreadCount}
              </span>
            )}
          </button>

          {bellOpen && (
            <div className="absolute right-0 mt-2 w-72 bg-gray-800 border border-gray-700 rounded shadow-lg z-10 flex flex-col max-h-96">
              {/* Linha 1: título */}
              <div className="px-3 pt-2.5 pb-0">
                <span className="text-xs font-semibold text-gray-300">Notificações</span>
              </div>

              {/* Linha 2: checkbox selecionar todos + kebab menu */}
              <div className="flex items-center justify-between px-3 py-1.5 border-b border-gray-700">
                <input
                  type="checkbox"
                  checked={allSelected}
                  ref={(el) => {
                    if (el) el.indeterminate = someSelected && !allSelected;
                  }}
                  onChange={toggleAll}
                  className="w-3 h-3 accent-blue-500 cursor-pointer"
                  title="Selecionar todas"
                />

                {/* Kebab (···) */}
                <div className="relative" ref={kebabRef}>
                  <button
                    onClick={() => setKebabOpen((v) => !v)}
                    disabled={notifications.length === 0}
                    className="flex items-center gap-0.5 text-gray-400 hover:text-white disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
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
                          onClick={() =>
                            ask({
                              title: 'Marcar como lidas',
                              message: `Marcar ${targetLabel()} como lidas?`,
                              confirmLabel: 'Marcar',
                              action: () => { markSelectedRead(targetIds()); setSelectedIds(new Set()) },
                            })
                          }
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
                          onClick={() =>
                            ask({
                              title: 'Marcar como não lidas',
                              message: `Marcar ${targetLabel()} como não lidas?`,
                              confirmLabel: 'Marcar',
                              action: () => { markAllUnread(targetIds()); setSelectedIds(new Set()) },
                            })
                          }
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
                        onClick={() =>
                          ask({
                            title: 'Excluir notificações',
                            message: `Excluir ${targetLabel()}? Esta ação não pode ser desfeita.`,
                            confirmLabel: 'Excluir',
                            danger: true,
                            action: () => {
                              if (someSelected) {
                                removeSelected(targetIds())
                              } else {
                                removeAll()
                              }
                              setSelectedIds(new Set())
                            },
                          })
                        }
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

              {/* Lista */}
              <div className="overflow-y-auto flex-1">
                {notifications.length === 0 ? (
                  <p className="text-xs text-gray-500 text-center py-6">
                    Nenhuma notificação
                  </p>
                ) : (
                  notifications.map((n) => (
                    <NotificationItem
                      key={n.id}
                      n={n}
                      checked={selectedIds.has(n.id)}
                      onToggle={() => toggleOne(n.id)}
                      onClick={() => {
                        markRead(n.id);
                        setBellOpen(false);
                        navigate(`/cameras/${n.cameraId}`, {
                          state: { eventTime: n.time },
                        });
                      }}
                      onRemove={() =>
                        ask({
                          title: 'Excluir notificação',
                          message: 'Excluir esta notificação?',
                          confirmLabel: 'Excluir',
                          danger: true,
                          action: () => {
                            remove(n.id);
                            setSelectedIds((prev) => {
                              const next = new Set(prev);
                              next.delete(n.id);
                              return next;
                            });
                          },
                        })
                      }
                    />
                  ))
                )}
              </div>
            </div>
          )}
        </div>

        {/* Dropdown de usuário */}
        <div className="relative" ref={userRef}>
          <button
            onClick={() => setUserOpen((v) => !v)}
            className="flex items-center gap-2 text-sm text-gray-300 hover:text-white transition-colors"
          >
            {username}
            <svg
              className={`w-4 h-4 transition-transform ${userOpen ? "rotate-180" : ""}`}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M19 9l-7 7-7-7"
              />
            </svg>
          </button>
          {userOpen && (
            <div className="absolute right-0 mt-2 w-36 bg-gray-800 border border-gray-700 rounded shadow-lg z-10">
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
        title={confirm?.title ?? ''}
        message={confirm?.message ?? ''}
        confirmLabel={confirm?.confirmLabel}
        danger={confirm?.danger}
        onConfirm={handleConfirm}
        onCancel={() => setConfirm(null)}
      />
    </header>
  );
}
