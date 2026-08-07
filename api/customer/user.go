package customer

import (
	"context"
	"github.com/gin-gonic/gin"
	"mall/api"
	"mall/common"
)

func (c *Ctrl) GetUserByToken(ctx context.Context, token string) (*common.UserInfo, error) {
	return c.user.GetUserByToken(ctx, token)
}

func (c *Ctrl) GetUserInfo(ctx *gin.Context) {
	user := api.GetUserFromCtx(ctx)
	resp, errno := c.user.GetUserInfo(ctx.Request.Context(), user)
	api.WriteResp(ctx, resp, errno)
}
