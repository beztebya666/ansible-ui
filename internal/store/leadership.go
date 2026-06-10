package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// leaderLockKey is an arbitrary fixed key for the scheduler's session-level
// Postgres advisory lock. Only the replica holding it runs the scheduler loop,
// so cron schedules / autorun / retention fire exactly once across replicas.
const leaderLockKey int64 = 0x616E7369_75690001 // "ansiui" + 1

// Leadership holds a pinned connection whose session owns the advisory lock. The
// lock is released when Release() is called OR the connection/process dies (so a
// crashed leader's lock is reclaimable by another replica).
type Leadership struct {
	conn *pgxpool.Conn
}

// TryAcquireLeadership attempts to become the scheduler leader. It returns a
// non-nil handle if this replica won the lock, (nil, nil) if another replica
// holds it, or (nil, err) on a database error.
func (s *Store) TryAcquireLeadership(ctx context.Context) (*Leadership, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	var got bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, leaderLockKey).Scan(&got); err != nil {
		conn.Release()
		return nil, err
	}
	if !got {
		conn.Release() // someone else leads; don't hold the pinned connection
		return nil, nil
	}
	return &Leadership{conn: conn}, nil
}

// Ping checks the leader connection is still alive (the lock is only held while it
// is). A failure means leadership was lost and the caller should step down.
func (l *Leadership) Ping(ctx context.Context) error {
	return l.conn.Ping(ctx)
}

// Release unlocks and returns the pinned connection to the pool.
func (l *Leadership) Release() {
	_, _ = l.conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, leaderLockKey)
	l.conn.Release()
}
