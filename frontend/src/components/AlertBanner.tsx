import { useAlert, useAlertState, type AlertType } from '../contexts/AlertContext'

const variantStyles: Record<AlertType, string> = {
  success: 'bg-green-900/40 border-green-700/60 text-green-200',
  error: 'bg-red-900/40 border-red-700/60 text-red-200',
  warning: 'bg-amber-900/40 border-amber-700/60 text-amber-200',
  info: 'bg-blue-900/40 border-blue-700/60 text-blue-200',
}

function VariantIcon({ type }: { type: AlertType }) {
  const common = 'w-5 h-5 shrink-0'
  switch (type) {
    case 'success':
      return (
        <svg className={common} viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
          <path fillRule="evenodd" d="M16.7 5.3a1 1 0 0 1 0 1.4l-7.5 7.5a1 1 0 0 1-1.4 0L3.3 9.7a1 1 0 1 1 1.4-1.4l3.3 3.3 6.8-6.8a1 1 0 0 1 1.4 0Z" clipRule="evenodd" />
        </svg>
      )
    case 'error':
      return (
        <svg className={common} viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
          <path fillRule="evenodd" d="M10 18a8 8 0 1 0 0-16 8 8 0 0 0 0 16Zm-1-5a1 1 0 1 0 2 0V7a1 1 0 1 0-2 0v6Zm1 2.5a1 1 0 1 0 0 2 1 1 0 0 0 0-2Z" clipRule="evenodd" />
        </svg>
      )
    case 'warning':
      return (
        <svg className={common} viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
          <path fillRule="evenodd" d="M8.3 2.8a2 2 0 0 1 3.4 0l6.5 11A2 2 0 0 1 16.5 17h-13a2 2 0 0 1-1.7-3.2l6.5-11ZM11 7a1 1 0 1 0-2 0v4a1 1 0 1 0 2 0V7Zm-1 6.5a1 1 0 1 0 0 2 1 1 0 0 0 0-2Z" clipRule="evenodd" />
        </svg>
      )
    case 'info':
      return (
        <svg className={common} viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
          <path fillRule="evenodd" d="M10 18a8 8 0 1 0 0-16 8 8 0 0 0 0 16Zm1-11.5a1 1 0 1 1-2 0 1 1 0 0 1 2 0ZM9 9a1 1 0 0 0 0 2v3a1 1 0 0 0 1 1h1a1 1 0 1 0 0-2v-3a1 1 0 0 0-1-1H9Z" clipRule="evenodd" />
        </svg>
      )
  }
}

export default function AlertBanner() {
  const alerts = useAlertState()
  const { dismissAlert } = useAlert()

  if (alerts.length === 0) return null

  return (
    <div className="flex flex-col gap-px">
      {alerts.map(a => (
        <div
          key={a.id}
          role="alert"
          className={`flex items-center gap-3 border-b px-6 py-3 text-sm ${variantStyles[a.type]}`}
        >
          <VariantIcon type={a.type} />
          <span className="flex-1 break-words">{a.message}</span>
          <button
            type="button"
            aria-label="Fechar alerta"
            onClick={() => dismissAlert(a.id)}
            className="shrink-0 text-current/70 hover:text-current transition-colors"
          >
            <svg className="w-4 h-4" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
              <path d="M6.4 5 5 6.4 8.6 10 5 13.6 6.4 15 10 11.4 13.6 15 15 13.6 11.4 10 15 6.4 13.6 5 10 8.6 6.4 5Z" />
            </svg>
          </button>
        </div>
      ))}
    </div>
  )
}
