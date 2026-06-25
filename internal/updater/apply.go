package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"camera/internal/release"
)

const markerName = ".camera-update.json"

// Marker registra um update aplicado e ainda não confirmado (em teste). Fica no
// diretório do binário; o binário novo, ao subir saudável, o remove (confirma).
type Marker struct {
	State        string `json:"state"`
	Attempts     int    `json:"attempts"`
	Target       string `json:"target"`
	BackupBinary string `json:"backup_binary"`
	DBSnapshot   string `json:"db_snapshot"`
	DBPath       string `json:"db_path"`
	FromVersion  string `json:"from_version"`
	ToVersion    string `json:"to_version"`
}

func markerPath(dir string) string { return filepath.Join(dir, markerName) }

// WriteMarker grava (ou sobrescreve) o marcador em dir.
func WriteMarker(dir string, m Marker) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(markerPath(dir), data, 0o644)
}

// ReadMarker lê o marcador de dir. O bool indica se ele existe.
func ReadMarker(dir string) (Marker, bool, error) {
	data, err := os.ReadFile(markerPath(dir))
	if os.IsNotExist(err) {
		return Marker{}, false, nil
	}
	if err != nil {
		return Marker{}, false, err
	}
	var m Marker
	if err := json.Unmarshal(data, &m); err != nil {
		return Marker{}, true, fmt.Errorf("marcador inválido: %w", err)
	}
	return m, true, nil
}

// ClearMarker remove o marcador (confirmação do update). Ausência não é erro.
func ClearMarker(dir string) error {
	if err := os.Remove(markerPath(dir)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Applier orquestra a aplicação do update (caminho self-replace). As operações
// efetivas são injetadas para permitir teste sem rede, FS de binário ou exec.
type Applier struct {
	Dir            string
	Target         string
	BaseURL        string
	DBPath         string
	CurrentVersion string

	Download func(ctx context.Context, url, sha256, dest string) error
	Snapshot func() (string, error)
	Replace  func(src, target, backup string) error
	Reexec   func(target string) error
}

// Apply baixa o binário da arquitetura, faz snapshot do DB, troca o binário,
// grava o marcador (só após a troca) e re-executa. Em sucesso, Reexec não
// retorna; qualquer erro antes do Replace não deixa efeitos.
func (a *Applier) Apply(ctx context.Context, m release.Manifest) error {
	asset, ok := AssetForArch(m)
	if !ok {
		return fmt.Errorf("nenhum binário para a arquitetura atual no manifesto")
	}

	newPath := filepath.Join(a.Dir, ".camera.new")
	if err := a.Download(ctx, AssetURL(a.BaseURL, asset.Name), asset.SHA256, newPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	snap, err := a.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot do db: %w", err)
	}

	backup := filepath.Join(a.Dir, "camera.old")
	if err := a.Replace(newPath, a.Target, backup); err != nil {
		return fmt.Errorf("troca do binário: %w", err)
	}

	marker := Marker{
		State:        "pending",
		Attempts:     0,
		Target:       a.Target,
		BackupBinary: backup,
		DBSnapshot:   snap,
		DBPath:       a.DBPath,
		FromVersion:  a.CurrentVersion,
		ToVersion:    m.Latest,
	}
	if err := WriteMarker(a.Dir, marker); err != nil {
		return fmt.Errorf("gravar marcador: %w", err)
	}

	return a.Reexec(a.Target)
}
