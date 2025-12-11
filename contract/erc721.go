package contract

import (
	"context"
	"math/big"
	"strings"

	"nft_trade/utils"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
)

// ERC721ABI ERC721合约基础ABI（仅包含safeTransferFrom方法）
const ERC721ABI = `[
	{
		"inputs": [
			{"internalType": "address", "name": "from", "type": "address"},
			{"internalType": "address", "name": "to", "type": "address"},
			{"internalType": "uint256", "name": "tokenId", "type": "uint256"}
		],
		"name": "safeTransferFrom",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`

// ERC721Transactor ERC721交易器
type ERC721Transactor struct {
	client       *ethclient.Client
	abi          abi.ABI
	contractAddr common.Address
	chainID      *big.Int
}

// NewERC721Transactor 创建ERC721交易器
func NewERC721Transactor(rpcUrl string, contractAddr string) (*ERC721Transactor, error) {
	// 连接区块链节点
	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		utils.Logger.Error("连接区块链节点失败", zap.String("rpcUrl", rpcUrl), zap.Error(err))
		return nil, err
	}

	// 解析ABI
	abiObj, err := abi.JSON(strings.NewReader(ERC721ABI))
	if err != nil {
		utils.Logger.Error("解析ABI失败", zap.Error(err))
		return nil, err
	}

	// 获取链ID
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		utils.Logger.Error("获取链ID失败", zap.Error(err))
		return nil, err
	}

	return &ERC721Transactor{
		client:       client,
		abi:          abiObj,
		contractAddr: common.HexToAddress(contractAddr),
		chainID:      chainID,
	}, nil
}

// SafeTransferFrom 执行ERC721安全转账
// params:
// - privateKey: 卖家私钥（生产环境需使用钱包签名，勿直接存储）
// - from: 卖家地址
// - to: 买家地址
// - tokenId: 代币ID
// return: 交易哈希、错误
func (e *ERC721Transactor) SafeTransferFrom(privateKey string, from, to, tokenId string) (string, error) {
	ctx := context.Background()

	// 解析私钥
	// 生产环境：使用钱包签名（如MetaMask），避免直接处理私钥
	key, err := crypto.HexToECDSA(strings.TrimPrefix(privateKey, "0x"))
	if err != nil {
		utils.Logger.Error("解析私钥失败", zap.Error(err))
		return "", err
	}

	// 构建交易授权
	auth, err := bind.NewKeyedTransactorWithChainID(key, e.chainID)
	if err != nil {
		utils.Logger.Error("构建交易授权失败", zap.Error(err))
		return "", err
	}

	// 转换TokenID为big.Int
	tokenID := new(big.Int)
	_, ok := tokenID.SetString(tokenId, 10)
	if !ok {
		utils.Logger.Error("转换TokenID失败", zap.String("tokenId", tokenId))
		return "", err
	}

	// 调用合约方法
	contract := bind.NewBoundContract(e.contractAddr, e.abi, e.client, e.client, e.client)
	tx, err := contract.Transact(auth, "safeTransferFrom", common.HexToAddress(from), common.HexToAddress(to), tokenID)
	if err != nil {
		utils.Logger.Error("执行safeTransferFrom失败", zap.Error(err))
		return "", err
	}

	// 等待交易上链（可选，也可异步监听）
	receipt, err := bind.WaitMined(ctx, e.client, tx)
	if err != nil {
		utils.Logger.Error("等待交易上链失败", zap.String("txHash", tx.Hash().Hex()), zap.Error(err))
		return "", err
	}

	if receipt.Status == 0 {
		utils.Logger.Error("交易执行失败（状态为0）", zap.String("txHash", tx.Hash().Hex()))
		return "", err
	}

	return tx.Hash().Hex(), nil
}
