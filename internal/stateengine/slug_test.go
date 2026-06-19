package stateengine

import "testing"

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"com pessoa saindo": "com-pessoa-saindo",
		"Câmera Ligada":     "camera-ligada",
		"  vazio  ":         "vazio",
		"Portão (Aberto)":   "portao-aberto",
		"já-slug":           "ja-slug",
		"AÇÃO São Paulo":    "acao-sao-paulo",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
	// idempotente
	if Slug(Slug("Com Pessoa Entrando")) != "com-pessoa-entrando" {
		t.Errorf("Slug não é idempotente")
	}
}

func TestFriendlyLabel(t *testing.T) {
	classes := []string{"vazio", "com pessoa entrando", "com pessoa saindo"}
	// slug do modelo novo → rótulo amigável
	if got := FriendlyLabel("com-pessoa-saindo", classes); got != "com pessoa saindo" {
		t.Errorf("slug→amigável falhou: %q", got)
	}
	// rótulo do modelo antigo (já amigável) → identidade
	if got := FriendlyLabel("com pessoa entrando", classes); got != "com pessoa entrando" {
		t.Errorf("amigável→amigável falhou: %q", got)
	}
	// sem correspondência → como veio
	if got := FriendlyLabel("outro", classes); got != "outro" {
		t.Errorf("fallback falhou: %q", got)
	}
}
