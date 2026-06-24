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

// Último evento escolhido no carrossel (frame + a data em que foi escolhido), por
// câmera. Ao reabrir, o picker volta para essa data e centraliza/destaca o frame.
export interface PickedEvent {
  frame: string
  date: string
}

function pickedKey(cameraId: string): string {
  return `state-event-picker-picked:${cameraId}`
}

// loadPicked lê o evento escolhido; null quando não há nada salvo ou é inválido.
export function loadPicked(cameraId: string): PickedEvent | null {
  try {
    const raw = localStorage.getItem(pickedKey(cameraId))
    if (raw) {
      const p = JSON.parse(raw)
      if (typeof p?.frame === 'string' && p.frame && typeof p?.date === 'string' && p.date) {
        return { frame: p.frame, date: p.date }
      }
    }
  } catch { /* ignora parse/storage */ }
  return null
}

export function savePicked(cameraId: string, picked: PickedEvent): void {
  try {
    localStorage.setItem(pickedKey(cameraId), JSON.stringify(picked))
  } catch { /* ignora storage indisponível */ }
}
