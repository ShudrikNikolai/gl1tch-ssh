// Package migrations хранит SQL-миграции goose
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
