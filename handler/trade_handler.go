package handler

import (
	"net/http"
	"strconv"

	"nft_trade/service"
	"nft_trade/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// TradeHandler 交易处理器
type TradeHandler struct {
	tradeService service.TradeService
}

// NewTradeHandler 创建交易处理器
func NewTradeHandler(tradeService service.TradeService) *TradeHandler {
	return &TradeHandler{
		tradeService: tradeService,
	}
}

// CreateSellOrder 创建出售订单
func (h *TradeHandler) CreateSellOrder(c *gin.Context) {
	var req service.CreateSellOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Logger.Error("参数绑定失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  err.Error(),
		})
		return
	}

	orderNo, err := h.tradeService.CreateSellOrder(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": gin.H{"order_no": orderNo},
	})
}

// MatchOrder 撮合订单（买家购买）
func (h *TradeHandler) MatchOrder(c *gin.Context) {
	var req service.MatchOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Logger.Error("参数绑定失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  err.Error(),
		})
		return
	}

	orderNo, err := h.tradeService.MatchOrder(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": gin.H{"order_no": orderNo},
	})
}

// GetTradeRecords 查询交易记录
func (h *TradeHandler) GetTradeRecords(c *gin.Context) {
	// 解析查询参数
	userAddr := c.Query("user_addr")
	nftAssetIDStr := c.Query("nft_asset_id")
	pageStr := c.Query("page")
	pageSizeStr := c.Query("page_size")

	// 转换类型
	nftAssetID, _ := strconv.ParseUint(nftAssetIDStr, 10, 64)
	page, _ := strconv.Atoi(pageStr)
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(pageSizeStr)
	if pageSize <= 0 {
		pageSize = 10
	}

	req := service.GetTradeRecordsReq{
		UserAddr:   userAddr,
		NFTAssetID: nftAssetID,
		Page:       page,
		PageSize:   pageSize,
	}

	records, total, err := h.tradeService.GetTradeRecords(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": gin.H{
			"list":      records,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}
