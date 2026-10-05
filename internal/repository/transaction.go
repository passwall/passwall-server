package repository

import "context"

// TxManager executes repository operations atomically. Repositories resolve the
// transaction from the context passed to each method.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(context.Context) error) error
}
