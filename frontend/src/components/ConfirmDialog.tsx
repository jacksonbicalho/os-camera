import type { ReactNode } from 'react'
import { useEscapeKey } from '../hooks/useEscapeKey'
import { Button } from '@/components/ui/button'

interface ConfirmDialogProps {
  open: boolean
  title: string
  message: string
  confirmLabel?: string
  cancelLabel?: string
  danger?: boolean
  children?: ReactNode
  onConfirm: () => void
  onCancel: () => void
}

export default function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel = 'Confirmar',
  cancelLabel = 'Cancelar',
  danger = false,
  children,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  useEscapeKey(onCancel, open)

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-gray-800 border border-gray-700 rounded-lg shadow-xl p-6 w-80 flex flex-col gap-4">
        <h3 className="text-sm font-semibold text-gray-100">{title}</h3>
        <p className="text-xs text-gray-400">{message}</p>
        {children}
        <div className="flex justify-end gap-3">
          <Button id="confirm-dialog-cancel" size="sm" variant="outline" onClick={onCancel}>
            {cancelLabel}
          </Button>
          <Button
            id="confirm-dialog-confirm"
            size="sm"
            variant={danger ? 'destructive' : 'default'}
            onClick={onConfirm}
          >
            {confirmLabel}
          </Button>
        </div>
      </div>
    </div>
  )
}
