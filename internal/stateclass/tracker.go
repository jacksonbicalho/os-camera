package stateclass

// Tracker confirma o estado de um classificador só após N leituras iguais
// consecutivas com prob ≥ threshold. Leituras abaixo do limiar são ignoradas
// (não contam nem zeram a sequência). Emite a mudança apenas quando o estado
// recém-confirmado difere do anterior — é o que evita "piscar" entre estados.
//
// Zero value não é usável; use NewTracker.
type Tracker struct {
	threshold      float64
	minConsecutive int
	confirmed      string
	candidate      string
	streak         int
}

// NewTracker cria um Tracker. minConsecutive < 1 é tratado como 1.
func NewTracker(threshold float64, minConsecutive int) *Tracker {
	if minConsecutive < 1 {
		minConsecutive = 1
	}
	return &Tracker{threshold: threshold, minConsecutive: minConsecutive}
}

// Observe alimenta uma leitura (label de topo + sua probabilidade) e retorna
// (changed, estado confirmado atual). changed=true só na transição.
func (t *Tracker) Observe(label string, prob float64) (changed bool, state string) {
	if prob < t.threshold {
		return false, t.confirmed // abaixo do limiar: ignora (não conta nem zera)
	}
	if label == t.candidate {
		t.streak++
	} else {
		t.candidate = label
		t.streak = 1
	}
	if t.streak >= t.minConsecutive && t.candidate != t.confirmed {
		t.confirmed = t.candidate
		return true, t.confirmed
	}
	return false, t.confirmed
}

// State retorna o estado confirmado atual ("" se nenhum ainda).
func (t *Tracker) State() string { return t.confirmed }
