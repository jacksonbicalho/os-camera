import AppLayout from '../components/AppLayout'

interface PlaceholderPageProps {
  /** Título da seção (ex: "Mapas"). */
  title: string
  /** Texto opcional abaixo do título. */
  description?: string
}

// Página placeholder para seções da nav rail ainda não implementadas (Mapas,
// Dispositivos, Usuários, Relatórios). Cada uma será substituída pela página
// real nas histórias seguintes do roadmap do redesign.
export default function PlaceholderPage({ title, description }: PlaceholderPageProps) {
  return (
    <AppLayout>
      <div id={`placeholder-${title.toLowerCase()}`} className="max-w-2xl">
        <h2 className="text-2xl font-bold text-foreground">{title}</h2>
        <p className="text-sm text-muted-foreground mt-2">
          {description ?? 'Esta seção ainda está em construção.'}
        </p>
      </div>
    </AppLayout>
  )
}
