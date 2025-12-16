package dao

import (
	"fmt"
	"nft_trade/model"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
)

var db *gorm.DB

// InitMySQL 初始化MySQL连接
func InitMySQL(dsn string) error {
	var err error
	db, err = gorm.Open("mysql", dsn)
	if err != nil {
		return err
	}
	// 自动迁移表
	db.AutoMigrate(&model.Order{}, &model.Trade{})
	return nil
}

// CreateOrder 创建订单
func CreateOrder(order *model.Order) error {
	return db.Create(order).Error
}

// UpdateOrder 更新订单
func UpdateOrder(order *model.Order) error {
	order.UpdatedAt = model.Now()
	return db.Save(order).Error
}

// GetOrderById 根据ID查询订单
func GetOrderById(orderId string) (*model.Order, error) {
	var order model.Order
	if err := db.Where("id = ?", orderId).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

// CreateTrade 创建交易记录
func CreateTrade(trade *model.Trade) error {
	return db.Create(trade).Error
}

// DeleteOrder 删除指定ID的订单（导出函数，首字母大写）
// 参数：orderId 订单ID
// 返回：错误信息
func DeleteOrder(orderId string) error {
	// 示例：若订单存储在数据库中
	if err := db.Where("id = ?", orderId).Delete(&model.Order{}).Error; err != nil {
		return fmt.Errorf("delete order failed: %w", err)
	}

	// 示例：若订单存储在Redis中
	// if err := rdb.Del(ctx, "order:"+orderId).Err(); err != nil {
	// 	return fmt.Errorf("delete order from redis failed: %w", err)
	// }

	return nil
}
