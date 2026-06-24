import type { ReactNode } from 'react'
import { Button } from '@/components/ui/button'

interface ListPanelProps {
  sortOrder: 'asc' | 'desc'
  onSortOrderChange: () => void
  hasMore: boolean
  onLoadMore: () => void
  loadingMore?: boolean
  emptyMessage: string
  empty: boolean
  scroll?: boolean
  children: ReactNode
}

export default function ListPanel({
  sortOrder,
  onSortOrderChange,
  hasMore,
  onLoadMore,
  loadingMore = false,
  emptyMessage,
  empty,
  scroll = true,
  children,
}: ListPanelProps) {
  return (
    <div className="flex flex-col flex-1 min-h-0">
      <div className="px-3 py-1.5 border-b border-gray-800 flex justify-end shrink-0">
        <Button variant="link" size="sm" onClick={onSortOrderChange} className="h-auto p-0 text-xs">
          {sortOrder === 'desc' ? '↓ Recente' : '↑ Antigo'}
        </Button>
      </div>
      <div className={`divide-y divide-gray-700 ${scroll ? 'flex-1 min-h-0 overflow-y-auto [&::-webkit-scrollbar]:w-1 [&::-webkit-scrollbar-track]:bg-transparent [&::-webkit-scrollbar-thumb]:bg-gray-700 [&::-webkit-scrollbar-thumb]:rounded-full' : ''}`}>
        {empty
          ? <p className="px-3 py-4 text-sm text-gray-500">{emptyMessage}</p>
          : children
        }
        {hasMore && (
          <div className="px-3 py-2">
            <Button variant="link" size="sm" onClick={onLoadMore} disabled={loadingMore} className="h-auto p-0 text-sm">
              {loadingMore ? 'Carregando...' : 'Carregar mais'}
            </Button>
          </div>
        )}
      </div>
    </div>
  )
}
