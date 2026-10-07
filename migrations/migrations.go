// Package migrations embeds the SQL schema files so the binary can apply
// them without access to the source tree.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
