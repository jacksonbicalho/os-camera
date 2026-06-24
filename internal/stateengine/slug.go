package stateengine

import "strings"

var deaccent = map[rune]rune{
	'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a',
	'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e',
	'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i',
	'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
	'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u',
	'ç': 'c', 'ñ': 'n', 'ý': 'y',
}

// Slug converte um rótulo de classe em um nome seguro (minúsculo, ASCII, hífens),
// usado como nome de diretório (state_samples/state_train) e como identidade da
// classe no treino do YOLO. O rótulo amigável (com espaços/acentos) continua no
// banco e na UI; a conversão slug↔rótulo é feita nas bordas (runner/handler).
func Slug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prev := false
	for _, r := range s {
		if mapped, ok := deaccent[r]; ok {
			r = mapped
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prev = false
		} else if !prev {
			b.WriteByte('-')
			prev = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// FriendlyLabel mapeia a classe devolvida pelo modelo de volta para o rótulo
// amigável do classificador. Casa tanto o slug (modelos novos) quanto o próprio
// rótulo (modelos antigos, treinados antes do slug). Sem correspondência, devolve
// o valor como veio.
func FriendlyLabel(predicted string, classes []string) string {
	for _, c := range classes {
		if c == predicted || Slug(c) == predicted {
			return c
		}
	}
	return predicted
}
