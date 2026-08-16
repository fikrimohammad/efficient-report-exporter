//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
)

// TestSeedLoadShops seeds a set of distinct shops for the load generator
// (cmd/loadgen). Each shop gets LOAD_ROWS report rows across the benchmark
// window, so the load generator can drive concurrent exports against distinct
// shops without colliding on the same keyset range.
//
// Configure via env (defaults in parentheses):
//
//	LOAD_SHOPS (16)   number of shops, seeded as [LOAD_START_SHOP, +N)
//	LOAD_ROWS  (100000) rows per shop
//	LOAD_START_SHOP (500000) first shop id
//
// Usage:
//
//	go test -tags integration -run TestSeedLoadShops ./integration/
func TestSeedLoadShops(t *testing.T) {
	ctx := context.Background()
	d := setupDeps(t)

	shops := 16
	if v := os.Getenv("LOAD_SHOPS"); v != "" {
		shops = atoi(v)
	}
	rowsPerShop := 100_000
	if v := os.Getenv("LOAD_ROWS"); v != "" {
		rowsPerShop = atoi(v)
	}
	startShop := int64(500000)
	if v := os.Getenv("LOAD_START_SHOP"); v != "" {
		startShop = int64(atoi(v))
	}

	start := benchStart
	end := benchStart.Add(benchWindow)

	for i := 0; i < shops; i++ {
		shopID := startShop + int64(i)
		if err := seedBenchRows(ctx, d.rawDB, shopID, rowsPerShop, start, end); err != nil {
			t.Fatalf("seed shop %d: %v", shopID, err)
		}
		t.Logf("seeded shop %d with %d rows", shopID, rowsPerShop)
	}
}
