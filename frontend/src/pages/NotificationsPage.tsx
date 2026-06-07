import { format, parseISO } from 'date-fns'
import { ptBR } from 'date-fns/locale'
import AppLayout from '../components/AppLayout'
import { Check, Trash2 } from '../components/Icons'
import { useUserNotifications, type UserNotificationType } from '../contexts/UserNotificationContext'

const variantDot: Record<UserNotificationType, string> = {
  success: 'bg-green-500',
  error: 'bg-red-500',
  warning: 'bg-amber-500',
  info: 'bg-blue-500',
}

function fmt(iso: string): string {
  try {
    return format(parseISO(iso), 'd MMM yyyy HH:mm', { locale: ptBR })
  } catch {
    return iso
  }
}

export default function NotificationsPage() {
  const { notifications, unreadCount, markRead, markAllRead, remove, removeAll } = useUserNotifications()

  return (
    <AppLayout>
      <div className="max-w-3xl mx-auto">
        <div className="flex items-start justify-between mb-6">
          <div>
            <h3 className="text-lg font-semibold text-gray-200">Notificações</h3>
            <p className="text-sm text-gray-500 mt-1">
              {unreadCount > 0 ? `${unreadCount} não lida(s)` : 'Tudo lido'}
            </p>
          </div>
          {notifications.length > 0 && (
            <div className="flex gap-2">
              <button
                onClick={markAllRead}
                disabled={unreadCount === 0}
                className="text-sm px-3 py-1.5 rounded border border-gray-700 text-gray-300 hover:bg-gray-800 disabled:opacity-40"
              >
                Marcar todas como lidas
              </button>
              <button
                onClick={removeAll}
                className="text-sm px-3 py-1.5 rounded border border-gray-700 text-red-400 hover:bg-gray-800"
              >
                Limpar tudo
              </button>
            </div>
          )}
        </div>

        {notifications.length === 0 ? (
          <p className="text-gray-500 text-sm">Nenhuma notificação.</p>
        ) : (
          <ul className="space-y-2">
            {notifications.map(n => (
              <li
                key={n.id}
                className={`flex items-start gap-3 rounded-lg border border-gray-800 px-4 py-3 ${n.read ? 'bg-gray-900/40' : 'bg-gray-900'}`}
              >
                <span className={`mt-1.5 w-2 h-2 rounded-full shrink-0 ${variantDot[n.type]}`} aria-hidden="true" />
                <div className="flex-1 min-w-0">
                  {n.title && <p className="text-sm font-medium text-gray-200">{n.title}</p>}
                  <p className="text-sm text-gray-300 break-words">{n.message}</p>
                  <p className="text-xs text-gray-500 mt-1">{fmt(n.created_at)}</p>
                </div>
                <div className="flex items-center gap-1 shrink-0">
                  {!n.read && (
                    <button
                      onClick={() => markRead(n.id)}
                      aria-label="Marcar como lida"
                      title="Marcar como lida"
                      className="p-1.5 rounded text-gray-400 hover:text-green-400 hover:bg-gray-800"
                    >
                      <Check className="w-4 h-4" />
                    </button>
                  )}
                  <button
                    onClick={() => remove(n.id)}
                    aria-label="Excluir"
                    title="Excluir"
                    className="p-1.5 rounded text-gray-400 hover:text-red-400 hover:bg-gray-800"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </AppLayout>
  )
}
