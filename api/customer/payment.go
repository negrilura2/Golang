package customer

import (
	"github.com/gin-gonic/gin"
	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/wechat/v3"
	"net/http"
)

func (c *Ctrl) WechatPaymentCallback(ctx *gin.Context) {
	notifyReq, err := wechat.V3ParseNotify(ctx.Request)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, wechat.V3NotifyRsp{
			Code:    gopay.FAIL,
			Message: err.Error(),
		})
		return
	}
	if err := c.order.WechatPaymentCallback(ctx.Request.Context(), notifyReq); err != nil {
		ctx.JSON(http.StatusInternalServerError, wechat.V3NotifyRsp{
			Code:    gopay.FAIL,
			Message: err.Error(),
		})
		return
	}
	ctx.JSON(http.StatusOK, wechat.V3NotifyRsp{
		Code: gopay.SUCCESS,
	})
}

func (c *Ctrl) WechatRefundCallback(ctx *gin.Context) {
	notifyReq, err := wechat.V3ParseNotify(ctx.Request)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, wechat.V3NotifyRsp{
			Code:    gopay.FAIL,
			Message: err.Error(),
		})
		return
	}
	if err := c.order.WechatRefundCallback(ctx.Request.Context(), notifyReq); err != nil {
		ctx.JSON(http.StatusInternalServerError, wechat.V3NotifyRsp{
			Code:    gopay.FAIL,
			Message: err.Error(),
		})
		return
	}
	ctx.JSON(http.StatusOK, wechat.V3NotifyRsp{
		Code: gopay.SUCCESS,
	})
}
