package clipboard

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type mutationLockTx struct {
	pgx.Tx
	lockStarted chan struct{}
	lockRelease chan struct{}
	lockErr     error
	userID      int64
	rollback    bool
	rollbackErr error
}

func (tx *mutationLockTx) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	if query != `SELECT pg_advisory_xact_lock($1::bigint)` || len(args) != 1 {
		panic("expected user lock before mutation SQL")
	}
	tx.userID = args[0].(int64)
	if tx.lockStarted != nil {
		close(tx.lockStarted)
	}
	if tx.lockRelease != nil {
		select {
		case <-tx.lockRelease:
		case <-ctx.Done():
			return pgconn.CommandTag{}, ctx.Err()
		}
	}
	return pgconn.CommandTag{}, tx.lockErr
}

func (tx *mutationLockTx) Rollback(ctx context.Context) error {
	tx.rollback = true
	tx.rollbackErr = ctx.Err()
	return nil
}

func TestBeginUserMutationWaitsForOwnerLockBeforeReturningTransaction(t *testing.T) {
	tx := &mutationLockTx{lockStarted: make(chan struct{}), lockRelease: make(chan struct{})}
	begin := func(context.Context) (pgx.Tx, error) { return tx, nil }
	type result struct {
		tx  pgx.Tx
		err error
	}
	ready := make(chan result, 1)
	go func() {
		locked, err := beginUserMutation(t.Context(), begin, 42)
		ready <- result{locked, err}
	}()
	<-tx.lockStarted
	select {
	case <-ready:
		t.Fatal("mutation may touch rows before its owner lock is acquired")
	default:
	}
	close(tx.lockRelease)
	got := <-ready
	if got.err != nil || got.tx != tx || tx.userID != 42 || tx.rollback {
		t.Fatalf("locked mutation = %#v, tx = %#v", got, tx)
	}
}

func TestBeginUserMutationRollsBackCanceledLockWait(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	tx := &mutationLockTx{lockStarted: make(chan struct{}), lockRelease: make(chan struct{})}
	begin := func(context.Context) (pgx.Tx, error) { return tx, nil }
	ready := make(chan error, 1)
	go func() {
		locked, err := beginUserMutation(ctx, begin, 42)
		if locked != nil {
			ready <- errors.New("canceled lock wait returned a usable mutation transaction")
			return
		}
		ready <- err
	}()
	<-tx.lockStarted
	cancel()
	if err := <-ready; !errors.Is(err, context.Canceled) {
		t.Fatalf("lock wait cancellation = %v", err)
	}
	if !tx.rollback || tx.rollbackErr != nil {
		t.Fatalf("failed lock wait must release transaction using uncanceled context: %#v", tx)
	}
}

func TestBeginUserMutationBeginAndLockFailures(t *testing.T) {
	failure := errors.New("database failure")
	if tx, err := beginUserMutation(t.Context(), func(context.Context) (pgx.Tx, error) { return nil, failure }, 1); tx != nil || !errors.Is(err, failure) {
		t.Fatalf("begin failure = %#v, %v", tx, err)
	}
	tx := &mutationLockTx{lockErr: failure}
	if got, err := beginUserMutation(t.Context(), func(context.Context) (pgx.Tx, error) { return tx, nil }, 1); got != nil || !errors.Is(err, failure) || !tx.rollback {
		t.Fatalf("lock failure = %#v, %v; rollback = %v", got, err, tx.rollback)
	}
}

func TestBuildListItemsQueryIncludesCreatedAtBounds(t *testing.T) {
	from := time.Date(2026, 8, 25, 2, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)

	query, args := buildListItemsQuery(42, ListFilter{
		Limit:         25,
		Query:         "  alpha  ",
		Kind:          KindImage,
		FavoriteOnly:  true,
		CreatedFrom:   &from,
		CreatedBefore: &to,
	})

	if !strings.Contains(query, "($5::timestamptz IS NULL OR created_at >= $5)") {
		t.Fatalf("query missing created_from bound: %s", query)
	}
	if !strings.Contains(query, "($6::timestamptz IS NULL OR created_at < $6)") {
		t.Fatalf("query missing created_before bound: %s", query)
	}
	if !strings.Contains(query, "LIMIT NULLIF($7, 0)") {
		t.Fatalf("query missing limit position: %s", query)
	}

	wantArgs := []any{int64(42), KindImage, true, "alpha", from, to, 25, nil, nil}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestBuildListItemsQueryLeavesOptionalBoundsNil(t *testing.T) {
	query, args := buildListItemsQuery(7, ListFilter{Limit: 10})

	if !strings.Contains(query, "($5::timestamptz IS NULL OR created_at >= $5)") || !strings.Contains(query, "($6::timestamptz IS NULL OR created_at < $6)") {
		t.Fatalf("query missing optional bound predicates: %s", query)
	}
	if len(args) != 9 {
		t.Fatalf("args length = %d, want 9", len(args))
	}
	if args[4] != nil || args[5] != nil || args[7] != nil || args[8] != nil {
		t.Fatalf("expected nil optional bounds, got %#v", args)
	}
}

func TestBuildListItemsQueryUsesExclusiveCursorTuple(t *testing.T) {
	createdAt := time.Date(2026, 11, 14, 10, 30, 0, 123456000, time.UTC)
	query, args := buildListItemsQuery(42, ListFilter{Cursor: &ListCursor{CreatedAt: createdAt, ID: 71}})
	for _, clause := range []string{
		"WHERE user_id = $1 AND deleted_at IS NULL",
		"($8::timestamptz IS NULL OR (created_at, id) < ($8, $9::bigint))",
		"ORDER BY created_at DESC, id DESC",
		"LIMIT NULLIF($7, 0)",
	} {
		if !strings.Contains(query, clause) {
			t.Fatalf("query missing %q: %s", clause, query)
		}
	}
	if args[6] != 0 || args[7] != createdAt || args[8] != int64(71) {
		t.Fatalf("cursor query must retain unlimited backup limit and exact cursor: %#v", args)
	}
}

func TestPostgresTrashQueriesAndAdditiveMigration(t *testing.T) {
	activeQuery, _ := buildListItemsQuery(42, ListFilter{})
	if !strings.Contains(activeQuery, "WHERE user_id = $1 AND deleted_at IS NULL") {
		t.Fatalf("normal list must exclude trash and scope ownership: %s", activeQuery)
	}
	trashQuery, args := buildListTrashQuery(42, 100)
	for _, clause := range []string{
		"WHERE user_id = $1 AND deleted_at IS NOT NULL",
		"ORDER BY deleted_at DESC, id DESC",
		"LIMIT NULLIF($2, 0)",
	} {
		if !strings.Contains(trashQuery, clause) {
			t.Fatalf("trash query missing %q: %s", clause, trashQuery)
		}
	}
	if !reflect.DeepEqual(args, []any{int64(42), 100}) {
		t.Fatalf("trash args = %#v", args)
	}
	if !strings.Contains(schema, "ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ") ||
		!strings.Contains(schema, "ADD COLUMN IF NOT EXISTS purge_pending BOOLEAN NOT NULL DEFAULT FALSE") ||
		!strings.Contains(schema, "CREATE INDEX IF NOT EXISTS idx_clipboard_items_user_trash") {
		t.Fatal("trash migration must be additive and safe to repeat")
	}
	if !strings.Contains(itemColumns, "deleted_at") || !strings.Contains(shareItemColumns, "i.deleted_at") {
		t.Fatal("item and share scans must carry deletion state")
	}
}
