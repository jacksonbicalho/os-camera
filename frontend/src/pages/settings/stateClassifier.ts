import type { BboxRect } from '../../components/BboxCanvas'

export interface StateClassifier {
  id?: number
  camera_id?: string
  name: string
  model?: string
  threshold: number
  trigger_motion: boolean
  trigger_interval_seconds: number
  crop_x: number
  crop_y: number
  crop_w: number
  crop_h: number
  min_consecutive: number
  enabled: boolean
  classes: string[]
  notify_enabled?: boolean
  footer_enabled?: boolean
  notify_user_ids?: number[]
  footer_user_ids?: number[]
}

type Crop = Pick<StateClassifier, 'crop_x' | 'crop_y' | 'crop_w' | 'crop_h'>

// bboxToCrop / cropToBbox convertem entre o retângulo do BboxCanvas (x,y,w,h 0–1)
// e os campos crop_* do classificador. A rotação não é usada no crop de estado.
export function bboxToCrop(b: BboxRect): Crop {
  return { crop_x: b.x, crop_y: b.y, crop_w: b.w, crop_h: b.h }
}

export function cropToBbox(c: Crop): BboxRect {
  return { x: c.crop_x, y: c.crop_y, w: c.crop_w, h: c.crop_h }
}

// validateClassifier espelha stateclass.Validate (Go). Retorna a mensagem de erro
// (pt-BR) ou null quando válido.
export function validateClassifier(c: StateClassifier): string | null {
  if (!c.name.trim()) return 'Nome é obrigatório'
  if (c.classes.filter(x => x.trim()).length < 2) return 'São necessárias ao menos 2 classes'
  if (
    c.crop_x < 0 || c.crop_y < 0 || c.crop_w <= 0 || c.crop_h <= 0 ||
    c.crop_x > 1 || c.crop_y > 1 || c.crop_x + c.crop_w > 1 || c.crop_y + c.crop_h > 1
  ) {
    return 'Recorte inválido: coordenadas fora de [0,1]'
  }
  if (!c.trigger_motion && c.trigger_interval_seconds <= 0) {
    return 'Defina ao menos um gatilho: movimento ou intervalo'
  }
  return null
}
