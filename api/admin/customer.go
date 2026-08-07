package admin

import (
	"github.com/gin-gonic/gin"
	"mall/api"
	"mall/common"
	"mall/service/dto"
)

func (c *Ctrl) CustomerUserList(ctx *gin.Context) {
	_ = api.GetAdminUserFromCtx(ctx)
	req := &dto.ListCustomerUserReq{}
	if err := ctx.BindQuery(req); err != nil {
		api.WriteResp(ctx, nil, common.ParamErr.WithErr(err))
		return
	}
	resp, errno := c.customerUser.CustomerUserList(ctx.Request.Context(), req)
	api.WriteResp(ctx, resp, errno)
}

func (c *Ctrl) GetCustomerUserInfo(ctx *gin.Context) {

}
