package customer

import (
	"github.com/gin-gonic/gin"
	"mall/api"
	"mall/common"
	"mall/service/dto"
)

func (c *Ctrl) MockPay(ctx *gin.Context) {
	_ = api.GetUserFromCtx(ctx)
	req := &dto.MockPayReq{}
	if err := ctx.BindJSON(req); err != nil {
		api.WriteResp(ctx, nil, common.ParamErr.WithErr(err))
		return
	}
	errno := c.order.MockPay(ctx.Request.Context(), req)
	api.WriteResp(ctx, nil, errno)
}
