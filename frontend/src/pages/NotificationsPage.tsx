import { format, parseISO } from 'date-fns'
import { ptBR } from 'date-fns/locale'
import { Link } from 'react-router-dom'
import AppLayout from '../components/AppLayout'
import { Check, Trash2 } from '../components/Icons'
import { useUserNotifications, type UserNotificationType } from '../contexts/UserNotificationContext'
import { Button } from '@/components/ui/button'

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
      <div>
        <div className="flex items-start justify-between mb-8">
          <div>
            <h2 className="text-2xl font-bold text-white">Notificações</h2>
            <p className="text-sm text-gray-500 mt-1">
              {unreadCount > 0 ? `${unreadCount} não lida(s)` : 'Tudo lido'}
            </p>
          </div>
          {notifications.length > 0 && (
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={markAllRead} disabled={unreadCount === 0}>
                Marcar todas como lidas
              </Button>
              <Button variant="outline" size="sm" onClick={removeAll} className="text-destructive hover:text-destructive">
                Limpar tudo
              </Button>
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
                  {n.link ? (
                    <Link
                      to={n.link}
                      onClick={() => markRead(n.id)}
                      className="text-sm text-blue-400 hover:text-blue-300 break-words underline-offset-2 hover:underline"
                    >
                      {n.message}
                    </Link>
                  ) : (
                    <p className="text-sm text-gray-300 break-words">{n.message}</p>
                  )}
                  <p className="text-xs text-gray-500 mt-1">{fmt(n.created_at)}</p>
                </div>
                <div className="flex items-center gap-1 shrink-0">
                  {!n.read && (
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => markRead(n.id)}
                      aria-label="Marcar como lida"
                      title="Marcar como lida"
                      className="h-8 w-8 text-muted-foreground hover:text-green-400"
                    >
                      <Check className="w-4 h-4" />
                    </Button>
                  )}
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => remove(n.id)}
                    aria-label="Excluir"
                    title="Excluir"
                    className="h-8 w-8 text-muted-foreground hover:text-destructive"
                  >
                    <Trash2 className="w-4 h-4" />
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </AppLayout>
  )
}
