package clipboard

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

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

	wantArgs := []any{int64(42), KindImage, true, "alpha", from, to, 25}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestBuildListItemsQueryLeavesOptionalBoundsNil(t *testing.T) {
	query, args := buildListItemsQuery(7, ListFilter{Limit: 10})

	if !strings.Contains(query, "($5::timestamptz IS NULL OR created_at >= $5)") || !strings.Contains(query, "($6::timestamptz IS NULL OR created_at < $6)") {
		t.Fatalf("query missing optional bound predicates: %s", query)
	}
	if len(args) != 7 {
		t.Fatalf("args length = %d, want 7", len(args))
	}
	if args[4] != nil || args[5] != nil {
		t.Fatalf("expected nil optional bounds, got %#v", args)
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
