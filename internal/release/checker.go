package release

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/mod/semver"
)

// DefaultDownloadBase é o diretório (redirect /latest/download do GitHub
// Releases) onde ficam o manifesto e os binários da última release.
const DefaultDownloadBase = "https://github.com/jacksonbicalho/os-camera/releases/latest/download/"

// DefaultManifestURL é o version.json da última release, servido estaticamente
// pelo GitHub Releases (sem a API).
const DefaultManifestURL = DefaultDownloadBase + "version.json"

// Status é um snapshot do estado da checagem, seguro para serializar.
type Status struct {
	Current         string    `json:"current"`
	Latest          string    `json:"latest"`
	NotesMD         string    `json:"notes_md"`
	Image           string    `json:"image"`
	UpdateAvailable bool      `json:"update_available"`
	CheckedAt       time.Time `json:"checked_at"`
	Err             string    `json:"error"`
}

// Checker busca periodicamente o manifesto remoto e cacheia o resultado.
// O valor zero não é utilizável; use NewChecker.
type Checker struct {
	url     string
	current string
	client  *http.Client

	mu        sync.RWMutex
	last      Manifest
	checkedAt time.Time
	lastErr   error
}

// NewChecker cria um Checker. client nil usa um http.Client com timeout de 10s.
func NewChecker(url, current string, client *http.Client) *Checker {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Checker{url: url, current: current, client: client}
}

// Check busca e parseia o manifesto, atualizando o cache (inclusive o erro em
// caso de falha) e retornando o resultado.
func (c *Checker) Check(ctx context.Context) (Manifest, error) {
	m, err := c.fetch(ctx)

	c.mu.Lock()
	c.checkedAt = time.Now()
	c.lastErr = err
	if err == nil {
		c.last = m
	}
	c.mu.Unlock()

	return m, err
}

func (c *Checker) fetch(ctx context.Context) (Manifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return Manifest{}, fmt.Errorf("montar request: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return Manifest{}, fmt.Errorf("buscar manifesto: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Manifest{}, fmt.Errorf("manifesto: status %d", resp.StatusCode)
	}
	var m Manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("decodificar manifesto: %w", err)
	}
	return m, nil
}

// Run checa uma vez na subida e depois a cada interval, até ctx ser cancelado.
// Erros de rede são resilientes: ficam no cache, sem derrubar a goroutine.
func (c *Checker) Run(ctx context.Context, interval time.Duration) {
	c.Check(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.Check(ctx)
		}
	}
}

// Manifest devolve o manifesto cacheado e se há um válido (ok=false antes de um
// check bem-sucedido).
func (c *Checker) Manifest() (Manifest, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.last, c.last.Latest != ""
}

// Status devolve um snapshot do cache.
func (c *Checker) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()

	st := Status{
		Current:         c.current,
		Latest:          c.last.Latest,
		NotesMD:         c.last.NotesMD,
		Image:           c.last.Image,
		UpdateAvailable: updateAvailable(c.current, c.last.Latest),
		CheckedAt:       c.checkedAt,
	}
	if c.lastErr != nil {
		st.Err = c.lastErr.Error()
	}
	return st
}

// updateAvailable é true só quando current e latest são semver válidos e
// latest é estritamente maior. Semver inválido (build dev sem tag) → false.
func updateAvailable(current, latest string) bool {
	if !semver.IsValid(current) || !semver.IsValid(latest) {
		return false
	}
	return semver.Compare(latest, current) > 0
}
