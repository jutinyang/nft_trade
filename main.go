package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"nft_trade/config"
	"nft_trade/handler"
	"nft_trade/model"
	"nft_trade/service"
	"nft_trade/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	// 1. 初始化配置
	if err := config.InitConfig(); err != nil {
		zap.L().Fatal("初始化配置失败", zap.Error(err))
	}

	// 2. 初始化日志
	if err := utils.InitLogger(); err != nil {
		zap.L().Fatal("初始化日志失败", zap.Error(err))
	}

	// 3. 初始化MySQL
	db, err := gorm.Open(mysql.Open(config.GlobalConfig.MySQLDSN), &gorm.Config{})
	if err != nil {
		utils.Logger.Fatal("连接MySQL失败", zap.Error(err))
	}

	// 自动迁移表结构（开发环境）
	err = db.AutoMigrate(
		&model.NFTAsset{},
		&model.NFTOrder{},
		&model.NFTAssetLock{},
		&model.NFTTradeRecord{},
	)
	if err != nil {
		utils.Logger.Fatal("迁移表结构失败", zap.Error(err))
	}

	// 4. 初始化Redis
	utils.InitRedis(config.GlobalConfig.RedisAddr, config.GlobalConfig.RedisPassword, config.GlobalConfig.RedisDB)

	// 5. 初始化RabbitMQ
	if err := utils.InitRabbitMQ(config.GlobalConfig.RabbitMQURL); err != nil {
		utils.Logger.Fatal("初始化RabbitMQ失败", zap.Error(err))
	}
	defer utils.CloseRabbitMQ()

	// 6. 初始化服务和处理器
	tradeService := service.NewTradeService(db)
	tradeHandler := handler.NewTradeHandler(tradeService)

	// 7. 启动RabbitMQ消费者（处理交易执行消息）
	err = utils.ConsumeTradeMsg(func(orderNo string) error {
		return tradeService.ExecuteTrade(context.Background(), orderNo)
	})
	if err != nil {
		utils.Logger.Fatal("启动消费者失败", zap.Error(err))
	}

	// 8. 初始化Gin引擎
	r := gin.Default()

	// 路由
	v1 := r.Group("/api/v1/trade")
	{
		v1.POST("/sell", tradeHandler.CreateSellOrder)   // 创建出售订单
		v1.POST("/match", tradeHandler.MatchOrder)       // 购买订单
		v1.GET("/records", tradeHandler.GetTradeRecords) // 查询交易记录
	}

	// 9. 启动服务（优雅关闭）
	go func() {
		if err := r.Run(config.GlobalConfig.ServerPort); err != nil {
			utils.Logger.Fatal("启动服务失败", zap.Error(err))
		}
	}()

	// 监听系统信号，优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	utils.Logger.Info("服务正在关闭...")
}
