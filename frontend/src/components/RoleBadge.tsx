// Badge da role do usuário no mesmo padrão dos demais badges do sistema
// (px-1.5 py-0.5 text-xs rounded + border — igual aos badges "motion"/"rec off"
// da lista de câmeras), em vez de um pill rounded-full próprio.
export default function RoleBadge({ role }: { role: string }) {
  return (
    <span
      className={`px-1.5 py-0.5 text-xs rounded border ${
        role === 'admin'
          ? 'bg-blue-900/40 text-blue-400 border-blue-800/50'
          : 'bg-surface-2 text-muted-foreground border-border'
      }`}
    >
      {role}
    </span>
  )
}
