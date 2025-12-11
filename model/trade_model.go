package model

import (
	"time"

	"gorm.io/gorm"
)

// NFTAsset NFT资产表（关联交易模块）
type NFTAsset struct {
	ID           uint64         `gorm:"primaryKey;comment:资产ID"`
	TokenID      string         `gorm:"uniqueIndex;comment:链上TokenID"`
	ContractAddr string         `gorm:"comment:NFT合约地址"`
	OwnerAddr    string         `gorm:"comment:当前持有者钱包地址"`
	MetadataCID  string         `gorm:"comment:IPFS元数据CID"`
	ChainID      int            `gorm:"comment:所属链ID"`
	Status       int            `gorm:"comment:0-正常 1-已销毁 2-冻结"`
	CreatedAt    time.Time      `gorm:"comment:创建时间"`
	UpdatedAt    time.Time      `gorm:"comment:更新时间"`
	DeletedAt    gorm.DeletedAt `gorm:"index;comment:删除时间"`
}

// NFTOrder NFT订单表（核心）
type NFTOrder struct {
	ID           uint64         `gorm:"primaryKey;comment:订单ID"`
	OrderNo      string         `gorm:"uniqueIndex;comment:订单编号（UUID）"`
	NFTAssetID   uint64         `gorm:"comment:关联NFT资产ID（外键）"`
	TokenID      string         `gorm:"comment:链上TokenID"`
	ContractAddr string         `gorm:"comment:NFT合约地址"`
	SellerAddr   string         `gorm:"comment:卖家钱包地址"`
	BuyerAddr    string         `gorm:"comment:买家钱包地址（未成交则为空）"`
	Price        string         `gorm:"comment:交易价格（wei单位）"`
	OrderType    int            `gorm:"comment:0-一口价 1-英式拍卖 2-荷兰式拍卖"`
	Status       int            `gorm:"comment:0-待成交 1-已成交 2-已取消 3-已过期 4-处理中 5-失败"`
	ChainID      int            `gorm:"comment:所属链ID"`
	StartTime    time.Time      `gorm:"comment:订单开始时间"`
	EndTime      time.Time      `gorm:"comment:订单结束时间"`
	CreatedAt    time.Time      `gorm:"comment:创建时间"`
	UpdatedAt    time.Time      `gorm:"comment:更新时间"`
	DeletedAt    gorm.DeletedAt `gorm:"index;comment:删除时间"`
}

// NFTAssetLock NFT资产锁定表（防止重复挂单）
type NFTAssetLock struct {
	ID         uint64         `gorm:"primaryKey;comment:锁定ID"`
	NFTAssetID uint64         `gorm:"uniqueIndex;comment:关联NFT资产ID"`
	OrderNo    string         `gorm:"comment:关联订单编号"`
	LockType   int            `gorm:"comment:0-交易挂单 1-拍卖"`
	LockTime   time.Time      `gorm:"comment:锁定时间"`
	UnlockTime *time.Time     `gorm:"comment:解锁时间（null表示未解锁）"`
	CreatedAt  time.Time      `gorm:"comment:创建时间"`
	UpdatedAt  time.Time      `gorm:"comment:更新时间"`
	DeletedAt  gorm.DeletedAt `gorm:"index;comment:删除时间"`
}

// NFTTradeRecord NFT交易记录表（最终账本）
type NFTTradeRecord struct {
	ID         uint64         `gorm:"primaryKey;comment:交易记录ID"`
	TradeNo    string         `gorm:"uniqueIndex;comment:交易编号（UUID）"`
	OrderNo    string         `gorm:"comment:关联订单编号"`
	NFTAssetID uint64         `gorm:"comment:关联NFT资产ID"`
	SellerAddr string         `gorm:"comment:卖家钱包地址"`
	BuyerAddr  string         `gorm:"comment:买家钱包地址"`
	Price      string         `gorm:"comment:交易价格"`
	Fee        string         `gorm:"comment:平台手续费"`
	FeeAddr    string         `gorm:"comment:手续费接收地址"`
	TxHash     string         `gorm:"comment:链上交易哈希（NFT转账）"`
	ChainID    int            `gorm:"comment:所属链ID"`
	TradeTime  time.Time      `gorm:"comment:交易完成时间"`
	CreatedAt  time.Time      `gorm:"comment:创建时间"`
	UpdatedAt  time.Time      `gorm:"comment:更新时间"`
	DeletedAt  gorm.DeletedAt `gorm:"index;comment:删除时间"`
}
