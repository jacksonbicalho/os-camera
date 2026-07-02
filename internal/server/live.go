package server

import (
	"encoding/json"
	"net/http"

	"github.com/pion/webrtc/v4"
)

// livePublisher negotiates a WebRTC session for a camera's live feed. Defined
// here (consumer side) to keep the server decoupled from the live package and
// testable with a fake; satisfied by *live.Publisher.
type livePublisher interface {
	Negotiate(offer webrtc.SessionDescription) (webrtc.SessionDescription, error)
}

// WithLivePublisher registers the WebRTC live publisher for a camera. Mirrors
// WithMonitor: cameras without a publisher (e.g. non-H.264 streams) simply have
// no WebRTC path and the client falls back to HLS.
func (s *Server) WithLivePublisher(cameraID string, p livePublisher) *Server {
	s.mu.Lock()
	if s.livePublishers == nil {
		s.livePublishers = make(map[string]livePublisher)
	}
	s.livePublishers[cameraID] = p
	s.mu.Unlock()
	return s
}

// handleWebRTC performs WHEP-style signaling for the low-latency live view: the
// client posts its SDP offer and receives the SDP answer. Cameras without a
// publisher return 409 so the client knows to fall back to HLS.
func (s *Server) handleWebRTC(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	s.mu.Lock()
	pub := s.livePublishers[id]
	s.mu.Unlock()
	if pub == nil {
		http.Error(w, "webrtc unavailable for this camera", http.StatusConflict)
		return
	}

	var body struct {
		SDP string `json:"sdp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SDP == "" {
		http.Error(w, "invalid offer", http.StatusBadRequest)
		return
	}

	answer, err := pub.Negotiate(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  body.SDP,
	})
	if err != nil {
		s.log.Warn("webrtc negotiation failed", "camera", id, "error", err)
		http.Error(w, "negotiation failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"sdp": answer.SDP})
}
