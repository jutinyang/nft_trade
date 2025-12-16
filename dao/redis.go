package dao

import (
	"context"
	"fmt"
	"nft_trade/model"
	"nft_trade/utils"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
)

// dao/redis.go
var (
	rdb = utils.RedisClient // 直接引用utils包导出的RedisClient
	ctx = context.Background()
)

// GetOrderBookKey 获取订单簿Key
// orderType: buy/sell, nftId: NFT资产ID
func GetOrderBookKey(orderType model.OrderType, nftId string) string {
	return fmt.Sprintf("nft:%s:%s", nftId, orderType)
}

// AddOrderToBook 将订单加入订单簿（ZSet）
// score: 价格（分） + 时间戳/1e12（保证价格相同时时间优先）
func AddOrderToBook(order *model.Order) error {
	// 时间戳从订单ID中提取（订单ID格式：{ts}-{uuid}）
	tsStr := strings.Split(order.ID, "-")[0]
	ts, _ := strconv.ParseInt(tsStr, 10, 64)
	// score = 价格 + 时间戳/1e12（确保价格相同时，时间早的订单score更小，排在前面）
	score := float64(order.Price) + float64(ts)/1e12
	if order.Type == model.OrderTypeBuy {
		// 买单：价格越高越优先，所以score取负数（ZSet升序排列时，负数越大越靠前）
		score = -score
	}
	return rdb.ZAdd(ctx, GetOrderBookKey(order.Type, order.NFTId), &redis.Z{
		Score:  score,
		Member: order.ID,
	}).Err()
}

// RemoveOrderFromBook 从订单簿移除订单
func RemoveOrderFromBook(order *model.Order) error {
	return rdb.ZRem(ctx, GetOrderBookKey(order.Type, order.NFTId), order.ID).Err()
}

// GetMatchableOrders 获取可匹配的订单（按价格优先排序）
// 例如：买单匹配卖单时，获取卖单簿中价格≤买单价格的订单
func GetMatchableOrders(buyOrder *model.Order) ([]string, error) {
	// 卖单簿Key
	sellBookKey := GetOrderBookKey(model.OrderTypeSell, buyOrder.NFTId)
	// 卖单score范围：0 ~ 买单价格（因为卖单score=价格+ts/1e12）
	maxScore := float64(buyOrder.Price) + 1e12 // 包含所有价格≤买单价格的卖单
	// 按score升序（价格从低到高）获取所有可匹配的卖单ID
	return rdb.ZRangeByScore(ctx, sellBookKey, &redis.ZRangeBy{
		Min: "0",
		Max: strconv.FormatFloat(maxScore, 'f', 12, 64),
	}).Result()
}

// GetOrderScore 获取订单在订单簿中的score
func GetOrderScore(order *model.Order) (float64, error) {
	return rdb.ZScore(ctx, GetOrderBookKey(order.Type, order.NFTId), order.ID).Result()
}
