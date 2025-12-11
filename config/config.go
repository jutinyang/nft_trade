package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config 全局配置
type Config struct {
	// MySQL配置
	MySQLDSN string
	// Redis配置
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	// RabbitMQ配置
	RabbitMQURL string
	// 区块链配置
	ChainRPCUrl map[int]string // 链ID -> RPC地址
	// 平台配置
	PlatformFeeRate float64 // 手续费比例（如0.02=2%）
	PlatformFeeAddr string  // 手续费接收地址
	ServerPort      string  // 服务端口
}

var GlobalConfig *Config

// InitConfig 初始化配置
func InitConfig() error {
	// 加载.env文件
	err := godotenv.Load()
	if err != nil {
		return err
	}

	// 初始化链RPC配置
	chainRPCUrl := make(map[int]string)
	// 以太坊测试网Sepolia
	chainRPCUrl[11155111] = getEnv("SEPOLIA_RPC_URL", "https://rpc.sepolia.org")
	// Polygon测试网Mumbai
	chainRPCUrl[80001] = getEnv("MUMBAI_RPC_URL", "https://rpc-mumbai.maticvigil.com")

	// 解析手续费比例
	feeRate, err := strconv.ParseFloat(getEnv("PLATFORM_FEE_RATE", "0.02"), 64)
	if err != nil {
		return err
	}

	// 解析Redis DB
	redisDB, err := strconv.Atoi(getEnv("REDIS_DB", "0"))
	if err != nil {
		return err
	}

	GlobalConfig = &Config{
		MySQLDSN:        getEnv("MYSQL_DSN", "root:123456@tcp(127.0.0.1:3306)/nft_db?charset=utf8mb4&parseTime=True&loc=Local"),
		RedisAddr:       getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:   getEnv("REDIS_PASSWORD", ""),
		RedisDB:         redisDB,
		RabbitMQURL:     getEnv("RABBITMQ_URL", "amqp://guest:guest@127.0.0.1:5672/"),
		ChainRPCUrl:     chainRPCUrl,
		PlatformFeeRate: feeRate,
		PlatformFeeAddr: getEnv("PLATFORM_FEE_ADDR", "0x0000000000000000000000000000000000000000"),
		ServerPort:      getEnv("SERVER_PORT", ":8080"),
	}

	return nil
}

// getEnv 获取环境变量，若不存在则返回默认值
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
