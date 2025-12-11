package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"nft_trade/config"
	"nft_trade/contract"
	"nft_trade/model"
	"nft_trade/utils"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// TradeService 交易服务接口
type TradeService interface {
	CreateSellOrder(ctx context.Context, req CreateSellOrderReq) (string, error)
	MatchOrder(ctx context.Context, req MatchOrderReq) (string, error)
	ExecuteTrade(ctx context.Context, orderNo string) error
	GetTradeRecords(ctx context.Context, req GetTradeRecordsReq) ([]model.NFTTradeRecord, int64, error)
}

// tradeService 交易服务实现
type tradeService struct {
	db *gorm.DB
}

// NewTradeService 创建交易服务
func NewTradeService(db *gorm.DB) TradeService {
	return &tradeService{
		db: db,
	}
}

// -------------- 请求结构体 --------------
// CreateSellOrderReq 创建出售订单请求
type CreateSellOrderReq struct {
	NFTAssetID uint64     `json:"nft_asset_id"`
	SellerAddr string     `json:"seller_addr"`
	Price      string     `json:"price"`
	OrderType  int        `json:"order_type"` // 0-一口价 1-英式拍卖 2-荷兰式拍卖
	ChainID    int        `json:"chain_id"`
	EndTime    *time.Time `json:"end_time"` // 可选，默认7天
}

// MatchOrderReq 撮合订单请求（买家购买）
type MatchOrderReq struct {
	OrderNo   string `json:"order_no"`
	BuyerAddr string `json:"buyer_addr"`
}

// GetTradeRecordsReq 查询交易记录请求
type GetTradeRecordsReq struct {
	UserAddr   string `json:"user_addr"` // 买家/卖家地址
	NFTAssetID uint64 `json:"nft_asset_id"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
}

// -------------- 核心方法 --------------
// CreateSellOrder 创建出售订单
func (s *tradeService) CreateSellOrder(ctx context.Context, req CreateSellOrderReq) (string, error) {
	// 1. 校验NFT资产是否存在且属于卖家
	var asset model.NFTAsset
	if err := s.db.WithContext(ctx).Where("id = ? AND owner_addr = ? AND status = 0", req.NFTAssetID, req.SellerAddr).First(&asset).Error; err != nil {
		utils.Logger.Error("校验NFT资产失败", zap.Uint64("nft_asset_id", req.NFTAssetID), zap.String("seller_addr", req.SellerAddr), zap.Error(err))
		return "", errors.New("NFT资产不存在或不属于当前用户，或资产状态异常")
	}

	// 2. 分布式锁：防止并发挂单（锁10秒）
	lockKey := fmt.Sprintf("nft_lock_%d", req.NFTAssetID)
	mutex, err := utils.GetRedisLock(ctx, lockKey, 10*time.Second)
	if err != nil {
		utils.Logger.Error("获取分布式锁失败", zap.String("lockKey", lockKey), zap.Error(err))
		return "", errors.New("当前资产正在处理中，请稍后再试")
	}
	defer utils.ReleaseRedisLock(mutex)

	// 3. 校验资产是否已被锁定
	var lockRecord model.NFTAssetLock
	if err := s.db.WithContext(ctx).Where("nft_asset_id = ? AND unlock_time IS NULL", req.NFTAssetID).First(&lockRecord).Error; err == nil {
		return "", errors.New("NFT资产已被锁定，无法挂单")
	}

	// 4. 构建订单
	orderNo := uuid.NewString()                   // 生成唯一订单号
	endTime := time.Now().Add(7 * 24 * time.Hour) // 默认7天
	if req.EndTime != nil {
		endTime = *req.EndTime
	}

	order := model.NFTOrder{
		OrderNo:      orderNo,
		NFTAssetID:   req.NFTAssetID,
		TokenID:      asset.TokenID,
		ContractAddr: asset.ContractAddr,
		SellerAddr:   req.SellerAddr,
		Price:        req.Price,
		OrderType:    req.OrderType,
		Status:       0, // 待成交
		ChainID:      req.ChainID,
		StartTime:    time.Now(),
		EndTime:      endTime,
	}

	// 5. 事务：创建订单 + 锁定资产
	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 创建订单
	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		utils.Logger.Error("创建订单失败", zap.Error(err))
		return "", err
	}

	// 锁定资产
	lockRecord = model.NFTAssetLock{
		NFTAssetID: req.NFTAssetID,
		OrderNo:    orderNo,
		LockType:   0, // 交易挂单
		LockTime:   time.Now(),
	}
	if err := tx.Create(&lockRecord).Error; err != nil {
		tx.Rollback()
		utils.Logger.Error("锁定资产失败", zap.Error(err))
		return "", err
	}

	tx.Commit()

	return orderNo, nil
}

// MatchOrder 撮合订单（买家购买）
func (s *tradeService) MatchOrder(ctx context.Context, req MatchOrderReq) (string, error) {
	// 1. 校验订单状态：待成交、未过期
	var order model.NFTOrder
	if err := s.db.WithContext(ctx).Where("order_no = ? AND status = 0 AND end_time > ?", req.OrderNo, time.Now()).First(&order).Error; err != nil {
		utils.Logger.Error("校验订单失败", zap.String("order_no", req.OrderNo), zap.Error(err))
		return "", errors.New("订单不存在或已失效")
	}

	// 2. 校验买家不能是卖家
	if order.SellerAddr == req.BuyerAddr {
		return "", errors.New("不能购买自己的订单")
	}

	// 3. 更新订单状态为处理中，填充买家地址
	if err := s.db.WithContext(ctx).Model(&order).Updates(map[string]interface{}{
		"buyer_addr": req.BuyerAddr,
		"status":     4, // 处理中
	}).Error; err != nil {
		utils.Logger.Error("更新订单状态失败", zap.String("order_no", req.OrderNo), zap.Error(err))
		return "", err
	}

	// 4. 发布消息到RabbitMQ，异步执行交易
	if err := utils.PublishTradeMsg(ctx, req.OrderNo); err != nil {
		// 回滚订单状态
		s.db.WithContext(ctx).Model(&order).Updates(map[string]interface{}{
			"buyer_addr": "",
			"status":     0,
		})
		utils.Logger.Error("发布交易消息失败", zap.String("order_no", req.OrderNo), zap.Error(err))
		return "", errors.New("发起交易失败，请稍后再试")
	}

	return req.OrderNo, nil
}

// ExecuteTrade 执行交易（链上交割）
func (s *tradeService) ExecuteTrade(ctx context.Context, orderNo string) error {
	// 1. 查询订单信息
	var order model.NFTOrder
	if err := s.db.WithContext(ctx).Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		utils.Logger.Error("查询订单失败", zap.String("order_no", orderNo), zap.Error(err))
		return err
	}

	// 2. 查询NFT资产信息
	var asset model.NFTAsset
	if err := s.db.WithContext(ctx).Where("id = ?", order.NFTAssetID).First(&asset).Error; err != nil {
		utils.Logger.Error("查询NFT资产失败", zap.Uint64("nft_asset_id", order.NFTAssetID), zap.Error(err))
		return err
	}

	// 3. 获取区块链RPC地址
	rpcUrl, ok := config.GlobalConfig.ChainRPCUrl[order.ChainID]
	if !ok {
		utils.Logger.Error("未配置链RPC地址", zap.Int("chain_id", order.ChainID))
		return errors.New("链配置不存在")
	}

	// 4. 初始化ERC721合约交易器
	transactor, err := contract.NewERC721Transactor(rpcUrl, order.ContractAddr)
	if err != nil {
		return err
	}

	// 5. 执行链上NFT转账（卖家→买家）
	// 注意：生产环境中，私钥不应直接存储，需通过钱包签名获取交易哈希
	// 此处为演示，假设从配置/钱包服务中获取卖家私钥
	sellerPrivateKey := "0x你的卖家私钥" // 替换为实际私钥（测试网）
	txHash, err := transactor.SafeTransferFrom(sellerPrivateKey, order.SellerAddr, order.BuyerAddr, order.TokenID)
	if err != nil {
		// 更新订单状态为失败
		s.db.WithContext(ctx).Model(&order).Update("status", 5)
		return err
	}

	// 6. 计算平台手续费
	feeRate := config.GlobalConfig.PlatformFeeRate
	priceBig, _ := new(big.Float).SetString(order.Price)
	feeBig := new(big.Float).Mul(priceBig, big.NewFloat(feeRate))
	fee := feeBig.Text('f', 0) // 手续费（wei单位）
	feeAddr := config.GlobalConfig.PlatformFeeAddr

	// 7. 事务：更新订单状态 + 解锁资产 + 更新NFT所有者 + 创建交易记录
	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 更新订单状态为已成交
	if err := tx.Model(&order).Update("status", 1).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 解锁资产
	unlockTime := time.Now()
	if err := tx.Model(&model.NFTAssetLock{}).Where("order_no = ?", orderNo).Update("unlock_time", &unlockTime).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 更新NFT资产所有者
	if err := tx.Model(&asset).Update("owner_addr", order.BuyerAddr).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 创建交易记录
	tradeNo := uuid.NewString()
	tradeRecord := model.NFTTradeRecord{
		TradeNo:    tradeNo,
		OrderNo:    orderNo,
		NFTAssetID: order.NFTAssetID,
		SellerAddr: order.SellerAddr,
		BuyerAddr:  order.BuyerAddr,
		Price:      order.Price,
		Fee:        fee,
		FeeAddr:    feeAddr,
		TxHash:     txHash,
		ChainID:    order.ChainID,
		TradeTime:  time.Now(),
	}
	if err := tx.Create(&tradeRecord).Error; err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()

	utils.Logger.Info("交易执行成功", zap.String("order_no", orderNo), zap.String("trade_no", tradeNo), zap.String("tx_hash", txHash))
	return nil
}

// GetTradeRecords 查询交易记录
func (s *tradeService) GetTradeRecords(ctx context.Context, req GetTradeRecordsReq) ([]model.NFTTradeRecord, int64, error) {
	var records []model.NFTTradeRecord
	var total int64

	// 构建查询条件
	query := s.db.WithContext(ctx).Model(&model.NFTTradeRecord{})
	if req.UserAddr != "" {
		query = query.Where("seller_addr = ? OR buyer_addr = ?", req.UserAddr, req.UserAddr)
	}
	if req.NFTAssetID > 0 {
		query = query.Where("nft_asset_id = ?", req.NFTAssetID)
	}

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (req.Page - 1) * req.PageSize
	if err := query.Offset(offset).Limit(req.PageSize).Order("trade_time DESC").Find(&records).Error; err != nil {
		return nil, 0, err
	}

	return records, total, nil
}
