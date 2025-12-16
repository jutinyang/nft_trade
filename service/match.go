package service

import (
	"fmt"
	"nft_trade/dao"
	"nft_trade/model"
	"nft_trade/utils"

	"time"
)

// MatchEngine 订单撮合引擎（以买单匹配卖单为例，卖单匹配买单逻辑对称）
func MatchEngine(newOrder *model.Order) {

	lock, err := utils.NewRedisLock("XXXX", "XXXX", 0)

	// 仅处理买单（卖单匹配买单逻辑类似）
	if newOrder.Type != model.OrderTypeBuy {
		return
	}

	// 1. 获取可匹配的卖单ID列表（价格≤买单价格，按价格从低到高、时间从早到晚排序）
	sellOrderIds, err := dao.GetMatchableOrders(newOrder)
	if err != nil {
		fmt.Printf("get matchable orders failed: %v\n", err)
		return
	}

	remainingQty := newOrder.RemainingQty // 买单剩余未成交数量
	buyOrder := newOrder

	for _, sellOrderId := range sellOrderIds {
		if remainingQty <= 0 {
			break // 买单已全部成交，退出循环
		}

		// 2. 获取卖单详情
		sellOrder, err := dao.GetOrderById(sellOrderId)
		if err != nil {
			fmt.Printf("get sell order failed: %v\n", err)
			continue
		}

		// 3. 分布式锁：防止并发撮合
		lockKey := fmt.Sprintf("lock:order:%s:%s", buyOrder.ID, sellOrder.ID)
		lockID, err := lock.Lock(lockKey, 10*time.Second)

		if err != nil {
			fmt.Printf("get match lock failed: %v\n", err)
			continue
		}
		// 修复defer语法+闭包陷阱
		defer func(key, id string) {
			if err := lock.Unlock(key, id); err != nil {
				fmt.Printf("unlock match lock failed: %v\n", err)
			}
		}(lockKey, lockID)
		// 4. 检查卖单状态（可能已被撤销/成交）
		if sellOrder.Status != model.OrderStatusPending && sellOrder.Status != model.OrderStatusPartially {
			// 从订单簿移除无效订单
			dao.RemoveOrderFromBook(sellOrder)
			continue
		}

		// 5. 计算成交数量
		tradeQty := remainingQty
		if sellOrder.RemainingQty < tradeQty {
			tradeQty = sellOrder.RemainingQty
		}

		// 6. 生成交易记录
		tradeId := utils.GenerateOrderId() // 复用订单ID生成器
		trade := &model.Trade{
			ID:            tradeId,
			BuyOrderId:    buyOrder.ID,
			SellOrderId:   sellOrder.ID,
			NFTId:         buyOrder.NFTId,
			TradePrice:    sellOrder.Price, // 成交价格为卖单价格（价格优先）
			TradeQuantity: tradeQty,
			BuyerAddr:     buyOrder.UserAddr,
			SellerAddr:    sellOrder.UserAddr,
		}
		if err := dao.CreateTrade(trade); err != nil {
			fmt.Printf("create trade failed: %v\n", err)
			continue
		}

		// 7. 更新订单剩余数量和状态
		// 7.1 更新买单
		buyOrder.RemainingQty -= tradeQty
		if buyOrder.RemainingQty == 0 {
			buyOrder.Status = model.OrderStatusCompleted
		} else {
			buyOrder.Status = model.OrderStatusPartially
		}
		dao.UpdateOrder(buyOrder)

		// 7.2 更新卖单
		sellOrder.RemainingQty -= tradeQty
		if sellOrder.RemainingQty == 0 {
			sellOrder.Status = model.OrderStatusCompleted
			// 从订单簿移除已成交卖单
			dao.RemoveOrderFromBook(sellOrder)
		} else {
			sellOrder.Status = model.OrderStatusPartially
			// 更新订单簿中的卖单（无需更新score，仅剩余数量变化）
		}
		dao.UpdateOrder(sellOrder)

		// 8. 触发链上资产划转（异步执行，调用智能合约）
		go transferAsset(trade)

		// 9. 更新剩余数量
		remainingQty = buyOrder.RemainingQty
	}

	// 10. 若买单仍有剩余，保留在订单簿中；否则移除
	if buyOrder.RemainingQty == 0 {
		dao.RemoveOrderFromBook(buyOrder)
	}
}

// transferAsset 链上资产划转（简化版：实际需调用智能合约）
func transferAsset(trade *model.Trade) {
	// 1. 调用NFT合约的transferFrom方法，将NFT从卖方转移到买方
	// 2. 调用资金合约的转账方法，将资金从买方转移到卖方
	// 3. 处理交易确认后的状态更新
	fmt.Printf("transfer asset: nft %s, buyer %s, seller %s, price %d\n", trade.NFTId, trade.BuyerAddr, trade.SellerAddr, trade.TradePrice)
}
