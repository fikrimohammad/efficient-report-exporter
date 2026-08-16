package model

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/bytedance/sonic"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
)

type Report struct {
	ID                  int64            `db:"id" json:"id"`
	ShopID              int64            `db:"shop_id" json:"shop_id"`
	OrderID             int64            `db:"order_id" json:"order_id"`
	OrderCreationTime   time.Time        `db:"order_creation_time" json:"order_creation_time"`
	OrderPaymentTime    time.Time        `db:"order_payment_time" json:"order_payment_time"`
	OrderSettlementTime time.Time        `db:"order_settlement_time" json:"order_settlement_time"`
	FeeID               int64            `db:"fee_id" json:"fee_id"`
	Details             ReportFeeDetails `db:"details" json:"details"`
	CreationTime        time.Time        `db:"creation_time" json:"creation_time"`
	UpdateTime          time.Time        `db:"update_time" json:"update_time"`
}

type ReportFeeDetails []ReportFeeDetail

func (rfds *ReportFeeDetails) Scan(src any) error {
	var source []byte
	switch src := src.(type) {
	case string:
		source = []byte(src)
	case []byte:
		source = src
	default:
		return errors.New("incompatible type for ReportFeeDetails")
	}

	return sonic.Unmarshal(source, rfds)
}

func (rfds *ReportFeeDetails) Value() (driver.Value, error) {
	return sonic.Marshal(rfds)
}

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
	OrderCreationTime   time.Time
	OrderPaymentTime    time.Time
	OrderSettlementTime time.Time
	FeeID               int64
	ReportFeeDetail
}

func (rl *ReportLine) ToCSVRow() []string {
	return []string{
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
	}
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

// ReportBatchFile is the CSV artifact of a single date-range batch, streamed
// into the zip stage. Name is the archive entry name; Reader carries the bytes.
type ReportBatchFile struct {
	Name   string
	Reader io.ReadCloser
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
