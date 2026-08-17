package model

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"time"

	"github.com/bytedance/sonic"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
)

type Report struct {
	ID                  int64  `db:"id" json:"id"`
	ShopID              int64  `db:"shop_id" json:"shop_id"`
	OrderID             int64  `db:"order_id" json:"order_id"`
	OrderCreationTime   int64  `db:"order_creation_time" json:"order_creation_time"`
	OrderPaymentTime    int64  `db:"order_payment_time" json:"order_payment_time"`
	OrderSettlementTime int64  `db:"order_settlement_time" json:"order_settlement_time"`
	FeeID               int64  `db:"fee_id" json:"fee_id"`
	Details             []byte `db:"details" json:"details"`
	CreationTime        int64  `db:"creation_time" json:"creation_time"`
	UpdateTime          int64  `db:"update_time" json:"update_time"`
}

type ReportFeeDetails []ReportFeeDetail

type ReportFeeDetail struct {
	OrderDetailID      int64   `json:"order_detail_id"`
	CategoryID         int64   `json:"category_id"`
	ProductID          int64   `json:"product_id"`
	ProductPriceAmount float64 `json:"product_price_amount"`
	PromoAmount        float64 `json:"promo_amount"`
	FeeBaseAmount      float64 `json:"fee_base_amount"`
	FeeFinalAmount     float64 `json:"fee_final_amount"`
}

type ReportLine struct {
	ShopID              int64
	OrderID             int64
	OrderCreationTime   int64
	OrderPaymentTime    int64
	OrderSettlementTime int64
	FeeID               int64
	ReportFeeDetail
}

// ReportBatch is a fixed time-range slice of a report export, processed as one
// unit of work in the batched pipeline.
type ReportBatch struct {
	ShopID    int64
	StartTime time.Time
	EndTime   time.Time
}

// EntryName returns the zip archive entry name for this batch.
func (b ReportBatch) EntryName() string {
	return fmt.Sprintf("batch_%s_%s.csv",
		b.StartTime.Format(constant.ReportBatchTimeFormat),
		b.EndTime.Format(constant.ReportBatchTimeFormat))
}

// ReportBatchFile is the deflate-compressed CSV artifact of a single
// date-range batch. Each batch worker compresses its own CSV in parallel and
// carries the pre-computed CRC32 and sizes, so the zip stage can write the
// entry via CreateRaw without re-compressing.
type ReportBatchFile struct {
	Name             string
	Data             []byte
	CRC32            uint32
	CompressedSize   uint64
	UncompressedSize uint64
}

type ExportReportJob struct {
	ID           int64                          `db:"id" json:"id,string"`
	RequestID    int64                          `db:"request_id" json:"request_id"`
	ShopID       int64                          `db:"shop_id" json:"shop_id"`
	StartTime    int64                          `db:"start_time" json:"start_time"`
	EndTime      int64                          `db:"end_time" json:"end_time"`
	Status       constant.ExportReportJobStatus `db:"status" json:"status"`
	Extra        ExportReportJobExtra           `db:"extra" json:"extra"`
	CreationTime int64                          `db:"creation_time" json:"creation_time"`
	UpdateTime   *int64                         `db:"update_time" json:"update_time,omitempty"`
}

type ExportReportJobExtra struct {
	ErrCode  *int    `json:"err_code,omitempty"`
	ErrMsg   *string `json:"err_msg,omitempty"`
	FileName *string `json:"file_name,omitempty"`
}

func (erje *ExportReportJobExtra) Scan(src any) error {
	var source []byte
	switch src := src.(type) {
	case string:
		source = []byte(src)
	case []byte:
		source = src
	default:
		return errors.New("incompatible type for ExportReportJobExtra")
	}

	return sonic.Unmarshal(source, erje)
}

func (erje ExportReportJobExtra) Value() (driver.Value, error) {
	return sonic.Marshal(erje)
}
