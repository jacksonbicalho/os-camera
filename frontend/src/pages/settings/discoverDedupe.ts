// Helpers do rastreamento (DiscoverPage): casar resultado descoberto com a câmera
// já cadastrada (por IP no rtsp_url) e derivar o que mostrar na coluna "Nome /
// Modelo" (Identidade do device-info, com fallback no nome do scan/cadastro).

export interface RegisteredCamera {
  id?: string
  name?: string
  rtsp_url?: string
}

// findRegisteredCamera devolve a câmera cadastrada cujo rtsp_url referencia o IP
// descoberto, ou null.
export function findRegisteredCamera(
  ip: string,
  cameras: RegisteredCamera[],
): RegisteredCamera | null {
  if (!ip) return null
  return cameras.find(c => typeof c.rtsp_url === 'string' && c.rtsp_url.includes(ip)) ?? null
}

// findRegisteredCameraName retorna o nome (ou id) da câmera já cadastrada com
// aquele IP — usado no badge "Já cadastrada como …". null quando não cadastrada.
export function findRegisteredCameraName(
  ip: string,
  cameras: RegisteredCamera[],
): string | null {
  const cam = findRegisteredCamera(ip, cameras)
  if (!cam) return null
  return cam.name || cam.id || ip
}

export interface IdentityLine {
  label: string
  value: string
}

// Campos da seção "Identidade" exibidos na coluna, em ordem. hardware/collector
// ficam de fora (não fazem parte de "Nome / Modelo").
const IDENTITY_FIELDS: [key: string, label: string][] = [
  ['model', 'Modelo'],
  ['serial', 'Serial'],
  ['vendor', 'Fabricante'],
  ['firmware', 'Firmware'],
]

// identityLines extrai os pares Identidade (label/valor) do device-info,
// preservando a ordem e omitindo os campos ausentes/vazios.
export function identityLines(values: Record<string, string>): IdentityLine[] {
  return IDENTITY_FIELDS
    .filter(([key]) => values[key])
    .map(([key, label]) => ({ label, value: values[key] }))
}

// discoveredDisplayName é o fallback de nome quando não há device-info: nome do
// scan (ONVIF) → nome da câmera cadastrada (por IP) → vazio.
export function discoveredDisplayName(
  result: { name?: string; ip: string },
  cameras: RegisteredCamera[],
): string {
  return result.name || findRegisteredCameraName(result.ip, cameras) || ''
}
