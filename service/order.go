package service

import (
	"fmt"
	"nft_trade/dao"
	"nft_trade/model"
	"nft_trade/utils"
	"strconv"
	"time"
)

// PlaceOrder 挂单
func PlaceOrder(nftId, userAddr string, price int64, quantity int64, orderType model.OrderType, signature string) (string, error) {

	lock, err := utils.NewRedisLock("XXXX", "XXXX", 0)

	// 1. 前置校验
	// 1.1 签名验签
	data := nftId + userAddr + strconv.FormatInt(price, 10) + strconv.FormatInt(quantity, 10) + string(orderType)
	if !utils.VerifySignature(userAddr, data, signature) {
		return "", fmt.Errorf("signature verify failed")
	}
	// 1.2 资产校验（简化版：实际需检查用户是否持有NFT/资金充足）
	if orderType == model.OrderTypeSell {
		// 检查用户是否持有该NFT且未被冻结
		if !checkUserNFTAvailable(userAddr, nftId) {
			return "", fmt.Errorf("user not own nft or nft is frozen")
		}
	} else {
		// 检查用户资金是否充足
		if !checkUserFundAvailable(userAddr, price*quantity) {
			return "", fmt.Errorf("user fund not enough")
		}
	}

	// 2. 分布式锁：防止并发挂单
	lockKey := fmt.Sprintf("lock:nft:%s:user:%s", nftId, userAddr)

	lockID, err := lock.Lock(lockKey, 10*time.Second)
	if err != nil {
		fmt.Printf("get match lock failed: %v\n", err)
	}
	// 修复defer语法+闭包陷阱
	defer func(key, id string) {
		if err := lock.Unlock(key, id); err != nil {
			fmt.Printf("unlock match lock failed: %v\n", err)
		}
	}(lockKey, lockID)
	// 3. 资产冻结（简化版：实际需更新Redis/MySQL中的资产状态）
	if orderType == model.OrderTypeSell {
		freezeUserNFT(userAddr, nftId, quantity)
	} else {
		freezeUserFund(userAddr, price*quantity)
	}

	// 4. 创建订单
	orderId := utils.GenerateOrderId()
	order := &model.Order{
		ID:       orderId,
		NFTId:    nftId,
		UserAddr: userAddr,
		Price:    price,
		Quantity: quantity,
		Type:     orderType,
		Status:   model.OrderStatusPending,
	}
	if err := dao.CreateOrder(order); err != nil {
		// 回滚资产冻结
		unfreezeAsset(userAddr, nftId, quantity, orderType)
		return "", fmt.Errorf("create order failed: %v", err)
	}

	// 5. 加入订单簿
	if err := dao.AddOrderToBook(order); err != nil {
		// 回滚订单和资产
		dao.DeleteOrder(orderId)
		unfreezeAsset(userAddr, nftId, quantity, orderType)
		return "", fmt.Errorf("add order to book failed: %v", err)
	}

	// 6. 触发撮合引擎（异步执行，避免阻塞）
	go MatchEngine(order)

	return orderId, nil
}

// CancelOrder 撤单
func CancelOrder(orderId, userAddr, signature string) error {
	lock, err := utils.NewRedisLock("XXXX", "XXXX", 0)
	// 1. 前置校验
	// 1.1 签名验签
	data := orderId + userAddr
	if !utils.VerifySignature(userAddr, data, signature) {
		return fmt.Errorf("signature verify failed")
	}
	// 1.2 查询订单
	order, err := dao.GetOrderById(orderId)
	if err != nil {
		return fmt.Errorf("order not found: %v", err)
	}
	// 1.3 校验订单归属
	if order.UserAddr != userAddr {
		return fmt.Errorf("user not owner of order")
	}
	// 1.4 校验订单状态
	if order.Status == model.OrderStatusCompleted || order.Status == model.OrderStatusCancelled {
		return fmt.Errorf("order status not allow cancel")
	}

	// 2. 分布式锁
	lockKey := fmt.Sprintf("lock:order:%s", orderId)
	lockID, err := lock.Lock(lockKey, 10*time.Second)
	if err != nil {
		fmt.Printf("get match lock failed: %v\n", err)
	}
	// 修复defer语法+闭包陷阱
	defer func(key, id string) {
		if err := lock.Unlock(key, id); err != nil {
			fmt.Printf("unlock match lock failed: %v\n", err)
		}
	}(lockKey, lockID)

	// 3. 资产解冻
	unfreezeAsset(userAddr, order.NFTId, order.RemainingQty, order.Type)

	// 4. 更新订单状态
	order.Status = model.OrderStatusCancelled
	if err := dao.UpdateOrder(order); err != nil {
		return fmt.Errorf("update order status failed: %v", err)
	}

	// 5. 从订单簿移除
	if err := dao.RemoveOrderFromBook(order); err != nil {
		return fmt.Errorf("remove order from book failed: %v", err)
	}

	return nil
}

// 以下为简化版辅助函数，实际需根据业务实现
func checkUserNFTAvailable(userAddr, nftId string) bool {
	// 检查用户是否持有该NFT且未被冻结
	return true
}

func checkUserFundAvailable(userAddr string, amount int64) bool {
	// 检查用户资金是否充足
	return true
}

func freezeUserNFT(userAddr, nftId string, quantity int64) {
	// 冻结用户NFT
}

func freezeUserFund(userAddr string, amount int64) {
	// 冻结用户资金
}

func unfreezeAsset(userAddr, nftId string, quantity int64, orderType model.OrderType) {
	// 解冻资产
	if orderType == model.OrderTypeSell {
		// 解冻NFT
	} else {
		// 解冻资金
	}
}
