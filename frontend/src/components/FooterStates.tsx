import { Link } from 'react-router-dom'
import { useFooterStates } from '../hooks/useFooterStates'

// FooterStates lista no rodapé os classificadores marcados pelo usuário como
// `nome: estado`. Cada item é link para o histórico daquele classificador
// (?history={cid} abre a view de Histórico na página de Estados). Quando o estado
// de um muda entre polls, pisca por ~1 s. Sem itens, não renderiza nada.
export default function FooterStates() {
  const { states, flashing } = useFooterStates()

  if (states.length === 0) return null

  return (
    <div id="footer-states" className="flex items-center gap-3 text-[11px] text-gray-500">
      {states.map(s => (
        <Link
          key={s.classifier_id}
          to={`/settings/cameras/states/${s.camera_id}?history=${s.classifier_id}`}
          id={`footer-state-${s.classifier_id}`}
          title={`Ver histórico de ${s.name}`}
          className="px-1.5 py-0.5 rounded hover:bg-gray-800/60 transition-colors"
          style={flashing.has(s.classifier_id) ? { animation: 'footer-state-flash 1s ease-out' } : undefined}
        >
          <span className="text-gray-400">{s.name}:</span>{' '}
          <span className="text-gray-200 font-mono">{s.state || '—'}</span>
        </Link>
      ))}
    </div>
  )
}
