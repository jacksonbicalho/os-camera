package discovery

// Result holds information about a discovered camera.
type Result struct {
	IP         string   `json:"ip"`
	Port       int      `json:"port"`
	ONVIF      bool     `json:"onvif"`
	ONVIFXAddr string   `json:"onvif_xaddr,omitempty"`
	Name       string   `json:"name,omitempty"`
	RTSPURLs   []string `json:"rtsp_urls,omitempty"`
	Services   []string `json:"services,omitempty"`
}
