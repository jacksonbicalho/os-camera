import { getToken } from '../../auth'

export interface EventItem { time: string; frame: string }

// eventCleanFrameURL aponta para o _frame.jpg LIMPO salvo pelo motion junto do
// _motion.jpg (mesmo instante exato, sem box/score) — derivado do nome do snapshot.
// É o frame preferido: bate com o thumb selecionado, sem a defasagem de re-extrair
// da gravação (-ss salta pro próximo keyframe).
export function eventCleanFrameURL(cameraId: string, ev: EventItem): string {
  const d = new Date(ev.time)
  const dateDir = `${d.getUTCFullYear()}/${String(d.getUTCMonth() + 1).padStart(2, '0')}/${String(d.getUTCDate()).padStart(2, '0')}`
  const clean = ev.frame.replace('_motion.jpg', '_frame.jpg')
  return `/recordings/${cameraId}/${dateDir}/${clean}?token=${getToken()}`
}
