## 目录结构
```
./
nft-trade/
├── cmd/
│   └── main.go                 // 程序入口
├── config/
│   └── config.go               // 配置加载
├── handler/
│   └── trade_handler.go        // API接口层
├── model/
│   └── trade_model.go          // 数据模型（GORM）
├── service/
│   └── trade_service.go        // 核心业务服务层
├── contract/
│   └── erc721.go               // ERC721合约交互封装
├── utils/
│   ├── redis_lock.go           // Redis分布式锁
│   ├── rabbitmq.go             // RabbitMQ消息队列
│   └── logger.go               // 日志工具
├── go.mod                      // 依赖管理
└── go.sum
```