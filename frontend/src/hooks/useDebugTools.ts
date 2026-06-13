import { useCallback, useState } from 'react'
import type { BboxRect } from '../components/BboxCanvas'

// Estado do painel de Debug da CameraPage e suas ferramentas efêmeras.
// `closeDebug` fecha o painel E reseta os dois checkboxes (gráfico de scores e
// "Analisar limiar"), limpando o overlay/score — senão o BboxCanvas do "Analisar
// limiar" continua sobre o vídeo após o painel fechar.
export function useDebugTools() {
  const [showDebug, setShowDebug] = useState(false)
  const [showDebugChart, setShowDebugChart] = useState(false)
  const [analyzeMode, setAnalyzeMode] = useState(false)
  const [analyzeBox, setAnalyzeBox] = useState<BboxRect | null>(null)
  const [analyzeScore, setAnalyzeScore] = useState<number | null>(null)

  const closeDebug = useCallback(() => {
    setShowDebug(false)
    setShowDebugChart(false)
    setAnalyzeMode(false)
    setAnalyzeBox(null)
    setAnalyzeScore(null)
  }, [])

  return {
    showDebug, setShowDebug, closeDebug,
    showDebugChart, setShowDebugChart,
    analyzeMode, setAnalyzeMode,
    analyzeBox, setAnalyzeBox,
    analyzeScore, setAnalyzeScore,
  }
}
