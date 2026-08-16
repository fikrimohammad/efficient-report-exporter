package model

import (
	"encoding/csv"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
)

// referenceCSVRow renders rl exactly as encoding/csv would, using the same
// per-field formatting as the pre-optimization ToCSVRow. It is the ground
// truth that MarshalCSV (which writes raw, unquoted columns) must match
// byte-for-byte.
func referenceCSVRow(rl ReportLine) string {
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write([]string{
		strconv.FormatInt(rl.ShopID, 10),
		strconv.FormatInt(rl.FeeID, 10),
		strconv.FormatInt(rl.OrderID, 10),
		rl.OrderCreationTime.Format(constant.ReportLineTimeFormat),
		rl.OrderPaymentTime.Format(constant.ReportLineTimeFormat),
		rl.OrderSettlementTime.Format(constant.ReportLineTimeFormat),
		strconv.FormatInt(rl.OrderDetailID, 10),
		strconv.FormatInt(rl.ProductID, 10),
		strconv.FormatInt(rl.CategoryID, 10),
		strconv.FormatFloat(rl.ProductPriceAmount, 'f', -1, 64),
		strconv.FormatFloat(rl.PromoAmount, 'f', -1, 64),
		strconv.FormatFloat(rl.FeeBaseAmount, 'f', -1, 64),
		strconv.FormatFloat(rl.FeeFinalAmount, 'f', -1, 64),
	})
	w.Flush()
	return b.String()
}

func TestMarshalCSV_MatchesEncodingCSV(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	cases := []ReportLine{
		{},
		{
			ShopID:              math.MaxInt64,
			FeeID:               42,
			OrderID:             1,
			OrderCreationTime:   ts,
			OrderPaymentTime:    ts,
			OrderSettlementTime: ts,
			ReportFeeDetail: ReportFeeDetail{
				OrderDetailID:      1,
				ProductID:          2,
				CategoryID:         3,
				ProductPriceAmount: 9.99,
				PromoAmount:        1,
				FeeBaseAmount:      8.99,
				FeeFinalAmount:     0.5,
			},
		},
		{
			ShopID:              -1,
			FeeID:               -2,
			OrderID:             -3,
			OrderCreationTime:   ts.Add(-24 * time.Hour),
			OrderPaymentTime:    ts.Add(-23 * time.Hour),
			OrderSettlementTime: ts.Add(-22 * time.Hour),
			ReportFeeDetail: ReportFeeDetail{
				OrderDetailID:      -4,
				ProductID:          -5,
				CategoryID:         -6,
				ProductPriceAmount: -0.5,
				PromoAmount:        -1.25,
				FeeBaseAmount:      -9.99,
				FeeFinalAmount:     -0.1,
			},
		},
		{
			ShopID:              100,
			FeeID:               10,
			OrderID:             1000,
			OrderCreationTime:   time.Date(2026, 8, 16, 22, 47, 59, 0, time.UTC),
			OrderPaymentTime:    time.Date(2026, 8, 16, 22, 48, 59, 0, time.UTC),
			OrderSettlementTime: time.Date(2026, 8, 16, 22, 49, 59, 0, time.UTC),
			ReportFeeDetail: ReportFeeDetail{
				OrderDetailID:      999999,
				ProductID:          123456,
				CategoryID:         654321,
				ProductPriceAmount: 0.1,
				PromoAmount:        123456.789,
				FeeBaseAmount:      1e20,
				FeeFinalAmount:     1e-7,
			},
		},
	}

	for i, c := range cases {
		got := string(c.MarshalCSV(nil)) + "\n"
		want := referenceCSVRow(c)
		if got != want {
			t.Errorf("case %d mismatch:\n got %q\nwant %q", i, got, want)
		}
	}
}
