package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

const ReportNameTimeFormat = "20060102"
const ReportLineTimeFormat = "2006-01-02 15:04:05"

type Report struct {
	ID                  int64            `json:"id"`
	ShopID              int64            `json:"shop_id"`
	OrderID             int64            `json:"order_id"`
	OrderCreationTime   time.Time        `json:"order_creation_time"`
	OrderPaymentTime    time.Time        `json:"order_payment_time"`
	OrderSettlementTime time.Time        `json:"order_settlement_time"`
	FeeID               int64            `json:"fee_id"`
	Details             ReportFeeDetails `json:"details"`
	CreationTime        time.Time        `json:"creation_time"`
	UpdateTime          time.Time        `json:"update_time"`
}

type ReportFeeDetails []ReportFeeDetail

func (rfds *ReportFeeDetails) Scan(src interface{}) error {
	var source []byte
	switch src.(type) {
	case string:
		source = []byte(src.(string))
	case []byte:
		source = src.([]byte)
	default:
		return errors.New("incompatible type for ReportFeeDetails")
	}

	return json.Unmarshal(source, rfds)
}

func (rfds *ReportFeeDetails) Value() (driver.Value, error) {
	return json.Marshal(rfds)
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
	OrderCreationTime   time.Time `json:"order_creation_time"`
	OrderPaymentTime    time.Time `json:"order_payment_time"`
	OrderSettlementTime time.Time `json:"order_settlement_time"`
	FeeID               int64
	ReportFeeDetail
}
