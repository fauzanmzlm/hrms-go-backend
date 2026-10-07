package domain

import "context"

// Transactor runs fn inside a single database transaction. Repository calls
// made with the ctx passed to fn join that transaction. The transaction is
// committed when fn returns nil and rolled back otherwise.
type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
