package zones

type Zone struct {
	X               float64 `json:"x"`
	Y               float64 `json:"y"`
	W               float64 `json:"w"`
	H               float64 `json:"h"`
	Type            string  `json:"type,omitempty"` // "exclude" | "detect"; "" → "exclude"
	Label           string  `json:"label,omitempty"`
	Threshold       float64 `json:"threshold,omitempty"`        // 0 = herdado da câmera
	CooldownSeconds int     `json:"cooldown_seconds,omitempty"` // 0 = herdado
	FPS             int     `json:"fps,omitempty"`              // 0 = herdado da câmera
	Scale           float64 `json:"scale,omitempty"`            // 0 ou 1 = sem downscale; 0.5 = metade da resolução da zona
	Color           string  `json:"color,omitempty"`            // hex como "#ef4444"; vazio = cor padrão por tipo
	RotationDeg     float64 `json:"rotation_deg,omitempty"`     // graus no sentido horário; 0 = sem rotação
}

// IsExclude retorna true para zonas de exclusão (type "" ou "exclude").
// Zonas "detect" retornam false: são excluídas do diff global mas avaliadas independentemente.
func (z Zone) IsExclude() bool {
	return z.Type == "" || z.Type == "exclude"
}
