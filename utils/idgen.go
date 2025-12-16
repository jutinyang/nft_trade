package utils

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// GenerateOrderId 生成订单ID：{时间戳毫秒}-{UUID后8位}
func GenerateOrderId() string {
	ts := time.Now().UnixMilli()
	uuidStr := uuid.New().String()
	shortUUID := uuidStr[len(uuidStr)-8:]
	return fmt.Sprintf("%d-%s", ts, shortUUID)
}
