package migrations

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestReleasedConversationDailyMemoryMigrationIsImmutable(t *testing.T) {
	if got := fmt.Sprintf("%x", sha256.Sum256([]byte(conversationDailyMemorySchemaSQL))); got != "bf9ea6ba14eb2a17f8ea7a17621dad6b6472654c6cdd140af151d253cb4cce4d" {
		t.Fatalf("0042 rewritten: %s", got)
	}
}
func TestPostgresActorFactsUpgradePreservesReleasedHeadAndReruns(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.alembic_version SET version_num=$1`, ActorFactsPreviousHead); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var head string
	if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != Head {
		t.Fatalf("head %s %v", head, err)
	}
}
