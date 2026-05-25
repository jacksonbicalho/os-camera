package storage

import (
	"context"
	"io"
)

// Drive is a one-way archive destination. Uploads are fire-and-forget archives;
// there is no download path back into the system.
type Drive interface {
	Upload(ctx context.Context, key string, r io.Reader, size int64) error
}
