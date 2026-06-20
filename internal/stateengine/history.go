package stateengine

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SaveHistoryFrame copia o crop grabbado (transitório, em storage/tmp) para
// {storagePath}/state_history/{cid}/{unixms}.jpg e devolve o caminho servível pelo
// handler estático ("/recordings/state_history/{cid}/{arquivo}"). Diferente do crop
// em tmp — que é sobrescrito a cada leitura —, este frame é durável e fica como o
// thumbnail daquela transição no histórico.
func SaveHistoryFrame(storagePath string, cid int64, srcPath string, ts time.Time) (string, error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("read crop: %w", err)
	}
	dir := filepath.Join(storagePath, "state_history", fmt.Sprintf("%d", cid))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%d.jpg", ts.UnixMilli())
	if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
		return "", err
	}
	return fmt.Sprintf("/recordings/state_history/%d/%s", cid, name), nil
}
