import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

interface PageHeaderProps {
  title: string
  /** Subtítulo: string ou nós (ex.: Relatórios tem duas linhas). */
  subtitle?: ReactNode
  /** Bloco de ações alinhado à direita. */
  actions?: ReactNode
  /** `page` = top-level (text-2xl); `section` = sub-páginas de Settings (text-h2). */
  size?: 'page' | 'section'
  id?: string
  className?: string
}

// PageHeader — cabeçalho padronizado das páginas: título + subtítulo opcional +
// ações à direita, com espaçamento consistente (gap título↔subtítulo mt-2, mb-6).
// Substitui os cabeçalhos ad-hoc repetidos em cada página.
export default function PageHeader({ title, subtitle, actions, size = 'page', id, className }: PageHeaderProps) {
  return (
    <div id={id} className={cn('flex items-start justify-between gap-4 mb-6', className)}>
      <div className="min-w-0">
        <h2 className={cn('text-foreground', size === 'section' ? 'text-h2 font-semibold' : 'text-2xl font-bold')}>
          {title}
        </h2>
        {subtitle != null && <div className="text-sm text-muted-foreground mt-2">{subtitle}</div>}
      </div>
      {actions && <div className="flex items-center gap-2 shrink-0">{actions}</div>}
    </div>
  )
}
