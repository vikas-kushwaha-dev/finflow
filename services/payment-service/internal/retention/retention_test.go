package retention

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type fakeDB struct {
	values []int64
	index  int
	calls  int
}

func (db *fakeDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	db.calls++
	value := db.values[db.index]
	db.index++
	return fakeRow{value: value}
}

type fakeRow struct{ value int64 }

func (r fakeRow) Scan(dest ...any) error {
	*(dest[0].(*int64)) = r.value
	return nil
}

func TestRunRuleDryRunOnlyCounts(t *testing.T) {
	db := &fakeDB{values: []int64{12}}
	result, err := RunRule(context.Background(), db, Rules(24*time.Hour, 48*time.Hour, 72*time.Hour)[0], time.Now(), 10, true)
	if err != nil {
		t.Fatalf("RunRule() error = %v", err)
	}
	if result.Eligible != 12 || result.Deleted != 0 || db.calls != 1 {
		t.Fatalf("result = %#v calls = %d", result, db.calls)
	}
}

func TestRunRuleDeletesInBoundedBatches(t *testing.T) {
	db := &fakeDB{values: []int64{12, 5, 5, 2}}
	result, err := RunRule(context.Background(), db, Rules(24*time.Hour, 48*time.Hour, 72*time.Hour)[0], time.Now(), 5, false)
	if err != nil {
		t.Fatalf("RunRule() error = %v", err)
	}
	if result.Eligible != 12 || result.Deleted != 12 || db.calls != 4 {
		t.Fatalf("result = %#v calls = %d", result, db.calls)
	}
}

func TestRunRuleValidatesPolicy(t *testing.T) {
	if _, err := RunRule(context.Background(), &fakeDB{}, Rule{Name: "invalid"}, time.Now(), 0, false); err == nil {
		t.Fatal("RunRule() error = nil")
	}
}
