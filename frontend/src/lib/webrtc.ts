import { getToken } from '../auth'

/**
 * Raised when the server has no WebRTC publisher for the camera (HTTP 409),
 * e.g. a non-H.264 stream. Signals the caller to fall back to HLS.
 */
export class WebRTCUnavailableError extends Error {
  constructor(message = 'webrtc unavailable for this camera') {
    super(message)
    this.name = 'WebRTCUnavailableError'
  }
}

interface NegotiateOptions {
  token?: string
  fetchFn?: typeof fetch
}

// waitForIceGathering resolves once the peer connection has finished gathering
// ICE candidates. Signaling here is a single offer/answer exchange (no trickle),
// so the offer must carry all candidates — mirrors the server side.
function waitForIceGathering(pc: RTCPeerConnection): Promise<void> {
  if (pc.iceGatheringState === 'complete') return Promise.resolve()
  return new Promise((resolve) => {
    const check = () => {
      if (pc.iceGatheringState === 'complete') {
        pc.removeEventListener('icegatheringstatechange', check)
        resolve()
      }
    }
    pc.addEventListener('icegatheringstatechange', check)
  })
}

/**
 * Runs the WHEP-style handshake for a camera's live feed: creates an offer,
 * posts it to the server and applies the returned answer to `pc`. Throws
 * WebRTCUnavailableError on HTTP 409 (no publisher) and Error on other
 * failures — both signal the caller to fall back to HLS. `fetchFn`/`token`
 * are injectable so tests run without network or a real RTCPeerConnection.
 */
export async function negotiateWebRTC(
  cameraId: string,
  pc: RTCPeerConnection,
  opts: NegotiateOptions = {},
): Promise<void> {
  const fetchFn = opts.fetchFn ?? fetch
  const token = opts.token ?? getToken()

  const offer = await pc.createOffer()
  await pc.setLocalDescription(offer)
  await waitForIceGathering(pc)

  const resp = await fetchFn(`/api/cameras/${cameraId}/webrtc`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ sdp: pc.localDescription?.sdp ?? offer.sdp }),
  })

  if (resp.status === 409) throw new WebRTCUnavailableError()
  if (!resp.ok) throw new Error(`webrtc signaling failed: ${resp.status}`)

  const answer = (await resp.json()) as { sdp: string }
  await pc.setRemoteDescription({ type: 'answer', sdp: answer.sdp })
}
