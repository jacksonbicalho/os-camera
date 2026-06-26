import { useStats } from '../hooks/useStats'
import FooterStates from './FooterStates'

interface StatusBarProps {
  version?: string
}

function pct(n: number): string {
  return n < 0 ? '—' : `${Math.round(n)}%`
}

function mbps(n: number): string {
  if (n < 0) return '—'
  return `${n < 10 ? n.toFixed(1) : Math.round(n)} Mbps`
}

// StatusBar é o rodapé enxuto do app (redesign do Escopo B): estado do sistema à
// esquerda (CPU/Memória/Rede), estados ao vivo dos classificadores no centro
// (FooterStates) e versão + conexão à direita. Faz poll de /api/stats via useStats.
export default function StatusBar({ version }: StatusBarProps) {
  const { stats, connected } = useStats()
  const cpu = stats?.cpu_percent ?? -1
  const memUsed = stats ? stats.sys_mem_total_bytes - stats.sys_mem_free_bytes : 0
  const memPct = stats && stats.sys_mem_total_bytes > 0
    ? (memUsed / stats.sys_mem_total_bytes) * 100
    : -1
  const net = stats?.net_mbps ?? -1

  return (
    <footer
      id="status-bar"
      className="shrink-0 border-t border-border bg-surface px-4 py-1.5 flex items-center gap-6 text-[11px] text-muted-foreground"
    >
      <div className="flex items-center gap-1.5">
        <span className={`w-2 h-2 rounded-full ${connected ? 'bg-green-500' : 'bg-gray-600'}`} />
        <span>Sistema operacional</span>
      </div>
      <span id="status-cpu">CPU <span className="text-foreground font-medium tabular-nums">{pct(cpu)}</span></span>
      <span id="status-mem">Memória <span className="text-foreground font-medium tabular-nums">{pct(memPct)}</span></span>
      <span id="status-net">Rede <span className="text-foreground font-medium tabular-nums">{mbps(net)}</span></span>

      <div className="flex-1 flex justify-center min-w-0">
        <FooterStates />
      </div>

      {version && (
        <span id="status-version">Versão <span className="text-foreground font-mono">{version}</span></span>
      )}
      <div id="status-connection" className="flex items-center gap-1.5">
        <span className={`w-2 h-2 rounded-full ${connected ? 'bg-green-500' : 'bg-red-500'}`} />
        <span>{connected ? 'Conectado' : 'Desconectado'}</span>
      </div>
    </footer>
  )
}
