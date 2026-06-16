// findRegisteredCameraName retorna o nome (ou id) da câmera já cadastrada cujo
// rtsp_url referencia o IP descoberto — usado para, no rastreamento, ocultar o
// botão "Adicionar" e avisar que a câmera já está cadastrada. Retorna null quando
// nenhuma câmera cadastrada usa esse IP.
export function findRegisteredCameraName(
  ip: string,
  cameras: { name?: string; id?: string; rtsp_url?: string }[],
): string | null {
  if (!ip) return null
  const cam = cameras.find(c => typeof c.rtsp_url === 'string' && c.rtsp_url.includes(ip))
  if (!cam) return null
  return cam.name || cam.id || ip
}
