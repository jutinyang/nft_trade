package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// VerifySignature 验证签名（简化版：实际需用ECDSA验证钱包签名）
// params: userAddr-用户地址, data-待签数据, signature-签名
func VerifySignature(userAddr, data, signature string) bool {
	// 模拟验签：实际需调用go-ethereum的crypto包验证
	hash := sha256.Sum256([]byte(data + userAddr))
	expectedSig := hex.EncodeToString(hash[:])
	return signature == expectedSig[:16] // 简化匹配
}
