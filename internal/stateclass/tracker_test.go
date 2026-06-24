package stateclass_test

import (
	"testing"

	"camera/internal/stateclass"
)

func TestTrackerConfirmsAfterNConsecutive(t *testing.T) {
	tr := stateclass.NewTracker(0.8, 3)
	if ch, _ := tr.Observe("aberto", 0.9); ch {
		t.Fatal("não deveria confirmar na 1ª leitura")
	}
	if ch, _ := tr.Observe("aberto", 0.9); ch {
		t.Fatal("não deveria confirmar na 2ª leitura")
	}
	ch, st := tr.Observe("aberto", 0.9)
	if !ch || st != "aberto" {
		t.Fatalf("esperava confirmar 'aberto' na 3ª, got changed=%v st=%q", ch, st)
	}
}

func TestTrackerNoChangeOnceConfirmed(t *testing.T) {
	tr := stateclass.NewTracker(0.8, 2)
	tr.Observe("aberto", 0.9)
	tr.Observe("aberto", 0.9) // confirma
	if ch, _ := tr.Observe("aberto", 0.9); ch {
		t.Fatal("não deveria re-emitir o mesmo estado já confirmado")
	}
}

func TestTrackerIgnoresBelowThreshold(t *testing.T) {
	tr := stateclass.NewTracker(0.8, 3)
	tr.Observe("aberto", 0.9)
	tr.Observe("aberto", 0.9)
	if ch, _ := tr.Observe("aberto", 0.5); ch { // ignorada (abaixo do limiar)
		t.Fatal("leitura abaixo do limiar não deveria confirmar")
	}
	ch, st := tr.Observe("aberto", 0.9) // 3ª válida → confirma
	if !ch || st != "aberto" {
		t.Fatalf("esperava confirmar após 3 válidas, got %v %q", ch, st)
	}
}

func TestTrackerFlipFlopDoesNotConfirm(t *testing.T) {
	tr := stateclass.NewTracker(0.8, 3)
	tr.Observe("aberto", 0.9)
	tr.Observe("fechado", 0.9) // troca candidato, zera sequência
	tr.Observe("aberto", 0.9)
	if tr.State() != "" {
		t.Fatalf("não deveria ter confirmado nada, estado=%q", tr.State())
	}
}

func TestTrackerTransitionBetweenStates(t *testing.T) {
	tr := stateclass.NewTracker(0.8, 2)
	tr.Observe("aberto", 0.9)
	tr.Observe("aberto", 0.9) // confirma aberto
	if ch, _ := tr.Observe("fechado", 0.9); ch {
		t.Fatal("1 leitura de fechado não confirma")
	}
	ch, st := tr.Observe("fechado", 0.9)
	if !ch || st != "fechado" {
		t.Fatalf("esperava transição para 'fechado', got %v %q", ch, st)
	}
}
