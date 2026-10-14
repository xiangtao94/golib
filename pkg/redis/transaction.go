package redis

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Tx is the transaction passed to Watch and WatchWithRetry callbacks.
// It preserves the command API of the embedded UniversalClient.
type Tx = goredis.Tx

// Pipeliner queues commands for a transaction or pipeline.
type Pipeliner = goredis.Pipeliner

// ErrTxConflict means the watched keys changed before the transaction committed.
// WatchWithRetry returns it when all conflict retries have been exhausted.
var ErrTxConflict = goredis.TxFailedErr

// WatchWithRetry watches keys and invokes fn, retrying only transaction conflicts.
// It makes at most 32 attempts with a cancellable linear backoff of 1–31 ms.
// Other callback, connection, and command errors are returned immediately.
//
// fn must read through tx and commit writes through tx.TxPipelined. It may run
// repeatedly: recompute decisions and reset captured result variables each time,
// and keep external side effects (such as sending messages) outside the callback.
// Returning nil without committing is allowed for a read-only decision.
// Redis transactions do not roll back commands after a runtime command error.
// In cluster mode all watched and written keys must share a hash slot.
func (r *Redis) WatchWithRetry(ctx context.Context, fn func(*Tx) error, keys ...string) error {
	const maxAttempts = 32
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := r.Watch(ctx, fn, keys...)
		if !errors.Is(err, ErrTxConflict) {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if attempt == maxAttempts-1 {
			return ErrTxConflict
		}
		timer := time.NewTimer(time.Duration(attempt+1) * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return ErrTxConflict
}
