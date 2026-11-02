package migrations

import "embed"

// Files contains the versioned SQL shipped with the application.
//
//go:embed *.sql
var Files embed.FS
