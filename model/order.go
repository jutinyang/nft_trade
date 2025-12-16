package model

import (
	"time"

	"github.com/jinzhu/gorm"
)

// OrderStatus 订单状态
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"   // 待匹配
	OrderStatusPartially OrderStatus = "partially" // 部分成交
	OrderStatusCompleted OrderStatus = "completed" // 已成交
	OrderStatusCancelled OrderStatus = "cancelled" // 已撤销
	OrderStatusFailed    OrderStatus = "failed"    // 失败
)

// OrderType 订单类型（买单/卖单）
type OrderType string

const (
	OrderTypeBuy  OrderType = "buy"  // 买单
	OrderTypeSell OrderType = "sell" // 卖单
)

// Order NFT订单模型
type Order struct {
	ID           string      `gorm:"primary_key;column:id" json:"id"`           // 订单ID（包含时间戳）
	NFTId        string      `gorm:"column:nft_id" json:"nft_id"`               // NFT资产ID
	UserAddr     string      `gorm:"column:user_addr" json:"user_addr"`         // 用户钱包地址
	Price        int64       `gorm:"column:price" json:"price"`                 // 挂单价格（分，避免浮点精度问题）
	Quantity     int64       `gorm:"column:quantity" json:"quantity"`           // 挂单数量（NFT通常为1，批量为多个）
	RemainingQty int64       `gorm:"column:remaining_qty" json:"remaining_qty"` // 剩余未成交数量
	Type         OrderType   `gorm:"column:type" json:"type"`                   // 订单类型
	Status       OrderStatus `gorm:"column:status" json:"status"`               // 订单状态
	CreatedAt    time.Time   `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time   `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt    *time.Time  `gorm:"column:deleted_at" json:"deleted_at"`
}

// Now 返回当前时间（导出函数，首字母大写）
func Now() time.Time {
	return time.Now() // 实际项目中可根据需求格式化（如转成数据库时间格式）
}

// TableName 表名
func (o *Order) TableName() string {
	return "nft_orders"
}

// BeforeCreate 创建前钩子（设置创建时间）
func (o *Order) BeforeCreate(tx *gorm.DB) error {
	o.CreatedAt = time.Now()
	o.UpdatedAt = time.Now()
	o.RemainingQty = o.Quantity // 初始剩余数量=挂单数量
	return nil
}
