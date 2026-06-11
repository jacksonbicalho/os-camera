import { Check, Trash2 } from './Icons'

interface EventsPanelHeaderProps {
  allSelected: boolean
  someSelected: boolean
  canMarkRead: boolean
  onToggleAll: () => void
  onMarkRead: () => void
  onDelete: () => void
}

/**
 * Header of the events bell panel: the "Eventos" title (a proper typographic
 * header — larger and stronger than the list rows it leads), the select-all
 * checkbox, and the contextual action row that replaces the old three-dots menu.
 * The action row (Marcar como lido / Excluir) appears below "Selecionar todos"
 * only when at least one event is selected. Purely presentational.
 */
export default function EventsPanelHeader({
  allSelected,
  someSelected,
  canMarkRead,
  onToggleAll,
  onMarkRead,
  onDelete,
}: EventsPanelHeaderProps) {
  return (
    <div className="border-b border-gray-700">
      <h2 id="events-panel-title" className="px-3 pt-2.5 pb-1 text-lg font-semibold text-gray-100">
        Eventos
      </h2>

      <label
        id="events-select-all-label"
        className="flex items-center gap-1.5 px-3 py-1.5 cursor-pointer select-none"
      >
        <input
          id="events-select-all"
          type="checkbox"
          checked={allSelected}
          ref={(el) => { if (el) el.indeterminate = someSelected && !allSelected }}
          onChange={onToggleAll}
          className="w-3 h-3 accent-blue-500 cursor-pointer"
        />
        <span className="text-xs text-gray-400">Selecionar todos</span>
      </label>

      {someSelected && (
        <div id="events-actions-row" className="flex items-center gap-4 px-3 py-1.5 border-t border-gray-700">
          {canMarkRead && (
            <button
              id="events-action-mark-read"
              onClick={onMarkRead}
              className="flex items-center gap-1.5 text-xs text-gray-300 hover:text-white transition-colors"
            >
              <Check className="w-3.5 h-3.5 text-blue-400" />
              Marcar como lido
            </button>
          )}
          <button
            id="events-action-delete"
            onClick={onDelete}
            className="flex items-center gap-1.5 text-xs text-red-400 hover:text-red-300 transition-colors"
          >
            <Trash2 className="w-3.5 h-3.5" />
            Excluir
          </button>
        </div>
      )}
    </div>
  )
}
