import { useState, useRef, useEffect } from "react";
import { useNavigate, NavLink, Link } from "react-router-dom";
import { clearToken } from "../auth";
import { useNotifications } from "../contexts/NotificationContext";
import { formatDistanceToNow } from "date-fns";
import { ptBR } from "date-fns/locale";
import type { Notification } from "../contexts/NotificationContext";

interface HeaderProps {
  username?: string;
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
  onClick,
  onRemove,
}: {
  n: Notification;
  onClick: () => void;
  onRemove: () => void;
}) {
  const relTime = formatDistanceToNow(new Date(n.time), {
    addSuffix: true,
    locale: ptBR,
  });

  return (
    <div
      className={`flex items-start gap-2 px-3 py-2 cursor-pointer hover:bg-gray-750 transition-colors ${
        !n.read ? "border-l-2 border-blue-500" : "border-l-2 border-transparent"
      }`}
      onClick={onClick}
    >
      <div className="flex-1 min-w-0">
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

  const { notifications, unreadCount, markRead, markAllRead, remove, removeAll } =
    useNotifications();

  const navigate = useNavigate();

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
            className="relative text-gray-400 hover:text-white transition-colors"
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
              <div className="flex items-center justify-between px-3 py-2 border-b border-gray-700 flex-shrink-0">
                <span className="text-xs font-semibold text-gray-300">Notificações</span>
                <div className="flex gap-2">
                  <button
                    onClick={markAllRead}
                    className="text-xs text-blue-400 hover:text-blue-300"
                  >
                    Marcar todas como lidas
                  </button>
                  <span className="text-gray-600">·</span>
                  <button
                    onClick={removeAll}
                    className="text-xs text-gray-400 hover:text-gray-300"
                  >
                    Limpar tudo
                  </button>
                </div>
              </div>

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
                      onClick={() => {
                        markRead(n.id)
                        setBellOpen(false)
                        navigate(`/cameras/${n.cameraId}`, {
                          state: { eventTime: n.time },
                        })
                      }}
                      onRemove={() => remove(n.id)}
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
    </header>
  );
}
