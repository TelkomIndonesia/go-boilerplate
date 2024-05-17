package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutboxLock(t *testing.T) {
	ctx := context.Background()
	p := tGetPostgres(t)

	var scan bool
	key := keyNameAsHash64(t.Name())

	conn, err := p.db.Conn(ctx)
	require.NoError(t, err, "should not return error")
	defer conn.Close()
	err = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&scan)
	require.NoError(t, err, "should not return error")
	assert.True(t, scan, "should obtain lock")

	conn1, err := p.db.Conn(ctx)
	require.NoError(t, err, "should not return error")
	defer conn1.Close()
	err = conn1.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&scan)
	require.NoError(t, err, "should not return error")
	assert.False(t, scan, "should not obtain lock")

	conn.Close()
	conn2, err := p.db.Conn(ctx)
	require.NoError(t, err, "should not return error")
	defer conn2.Close()
	err = conn2.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&scan)
	require.NoError(t, err, "should not return error")
	assert.True(t, scan, "should obtain lock after the first one is closed")
}
