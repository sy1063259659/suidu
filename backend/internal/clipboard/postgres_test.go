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
