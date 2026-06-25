package updater

import (
	"fmt"

	"camera/internal/dbbackup"
)

// BootAction é a decisão tomada no boot quando há um marcador de update.
type BootAction int

const (
	// ActionTrial: primeiro boot pós-update; subir e, se saudável, confirmar.
	ActionTrial BootAction = iota
	// ActionRollback: o trial anterior não confirmou (subiu quebrado e foi
	// reiniciado) → reverter.
	ActionRollback
)

// EvaluateBoot decide o que fazer no boot a partir do marcador e devolve o
// marcador atualizado a persistir no caso de trial. attempts==0 é o trial (vira
// 1); attempts>=1 significa que o trial não confirmou → rollback.
func EvaluateBoot(m Marker) (BootAction, Marker) {
	if m.Attempts == 0 {
		m.Attempts = 1
		return ActionTrial, m
	}
	return ActionRollback, m
}

// Rollback restaura o binário anterior e o snapshot do DB pareado. O caller deve
// limpar o marcador e re-executar o binário restaurado em seguida.
func Rollback(m Marker) error {
	if err := copyFile(m.BackupBinary, m.Target); err != nil {
		return fmt.Errorf("restaurar binário: %w", err)
	}
	if err := dbbackup.Restore(m.DBSnapshot, m.DBPath); err != nil {
		return fmt.Errorf("restaurar db: %w", err)
	}
	return nil
}
