// Persistência (por câmera) da POSIÇÃO de rolagem do carrossel de eventos do
// classificador de estado, para "abrir de onde parei". Não envolve a data (o picker
// sempre abre em hoje). Guardado em localStorage, sobrevive a reabrir o classificador
// e a reload da página.

function key(cameraId: string): string {
  return `state-event-picker-scroll:${cameraId}`
}

// loadPickerScroll lê a rolagem salva; 0 quando não há nada salvo ou é inválido.
export function loadPickerScroll(cameraId: string): number {
  try {
    const n = Number(localStorage.getItem(key(cameraId)))
    return Number.isFinite(n) && n > 0 ? n : 0
  } catch {
    return 0
  }
}

export function savePickerScroll(cameraId: string, scrollLeft: number): void {
  try {
    localStorage.setItem(key(cameraId), String(Math.max(0, Math.round(scrollLeft))))
  } catch { /* ignora storage indisponível */ }
}
