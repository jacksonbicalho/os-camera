import { useFooterStates } from '../hooks/useFooterStates'

// FooterStates lista no rodapé os classificadores marcados pelo usuário como
// `nome: estado`. Quando o estado de um muda entre polls, pisca por ~1 s. Sem
// itens, não renderiza nada — não polui o rodapé.
export default function FooterStates() {
  const { states, flashing } = useFooterStates()

  if (states.length === 0) return null

  return (
    <div id="footer-states" className="flex items-center gap-3 text-[11px] text-gray-500">
      {states.map(s => (
        <span
          key={s.classifier_id}
          id={`footer-state-${s.classifier_id}`}
          className="px-1.5 py-0.5 rounded"
          style={flashing.has(s.classifier_id) ? { animation: 'footer-state-flash 1s ease-out' } : undefined}
        >
          <span className="text-gray-400">{s.name}:</span>{' '}
          <span className="text-gray-200 font-mono">{s.state || '—'}</span>
        </span>
      ))}
    </div>
  )
}
