import type { ReactNode } from 'react'

interface PlayerTitleProps {
  /** True quando o player está em modo ao vivo (não reprodução). */
  isLive: boolean
  /** Nome da câmera (ou id como fallback). */
  name: string
  /** Subtítulo exibido em reprodução (data/duração/label do evento). */
  subtitle?: ReactNode
}

// PlayerTitle é a identidade do header do player (redesign do Escopo B): ponto
// verde pulsante + nome + badge "AO VIVO" quando ao vivo; em reprodução, badge
// "Reprodução" e o subtítulo com data/duração.
export default function PlayerTitle({ isLive, name, subtitle }: PlayerTitleProps) {
  return (
    <div id="player-title" className="flex items-center gap-2 min-w-0">
      {isLive && (
        <span
          id="live-indicator"
          className="w-2 h-2 rounded-full bg-green-500 animate-pulse shrink-0"
          aria-hidden="true"
        />
      )}
      <span className="font-semibold text-sm text-foreground truncate">{name}</span>
      {isLive ? (
        <span
          id="live-badge"
          className="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-bold leading-none bg-green-500/15 text-green-400 shrink-0"
        >
          AO VIVO
        </span>
      ) : (
        <span
          id="playback-badge"
          className="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-bold leading-none bg-surface text-muted shrink-0"
        >
          Reprodução
        </span>
      )}
      {!isLive && subtitle && (
        <span className="text-sm text-muted min-w-0 truncate">{subtitle}</span>
      )}
    </div>
  )
}
