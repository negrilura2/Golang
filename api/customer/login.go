package customer

import (
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"mall/api"
	"mall/common"
	"mall/consts"
	"mall/service/dto"
)

func (c *Ctrl) GetSmsCodeCaptcha(ctx *gin.Context) {
	req := &dto.GetVerifyCaptchaReq{}
	if err := ctx.BindQuery(req); err != nil {
		api.WriteResp(ctx, nil, common.ParamErr.WithErr(err))
		return
	}
	pass := req.CheckSign()
	if !pass {
		api.WriteResp(ctx, nil, common.ParamErr)
		return
	}
	resp, errno := c.user.GetSlideCaptcha(ctx.Request.Context())
	api.WriteResp(ctx, resp, errno)
}

func (c *Ctrl) CheckSmsCodeCaptcha(ctx *gin.Context) {
	req := &dto.CheckCaptchaReq{}
	if err := ctx.BindJSON(req); err != nil {
		api.WriteResp(ctx, nil, common.ParamErr.WithErr(err))
		return
	}
	resp, errno := c.user.CheckSlideCaptcha(ctx.Request.Context(), req)
	api.WriteResp(ctx, resp, errno)
}

func (c *Ctrl) GetSmsVerifyCode(ctx *gin.Context) {
	req := &dto.GetSmsVerifyCodeReq{}
	if err := ctx.BindJSON(req); err != nil {
		api.WriteResp(ctx, nil, common.ParamErr.WithErr(err))
		return
	}
	errno := c.user.GetSmsVerifyCode(ctx.Request.Context(), req)
	api.WriteResp(ctx, nil, errno)
}

func (c *Ctrl) AppletLogin(ctx *gin.Context) {
	req := &dto.AppletLoginReq{}
	if err := ctx.BindJSON(req); err != nil {
		api.WriteResp(ctx, nil, common.ParamErr.WithErr(err))
		return
	}
	if req.Code == "" || !lo.Contains([]int64{consts.WechatAppCode, consts.AppletAppCode}, int64(req.AppCode)) {
		api.WriteResp(ctx, nil, common.ParamErr.WithMsg("AppCode error or code is empty"))
		return
	}
	resp, errno := c.user.AppletLogin(ctx.Request.Context(), req)
	api.WriteResp(ctx, resp, errno)
}

func (c *Ctrl) MobilePasswordLogin(ctx *gin.Context) {
	req := &dto.MobilePasswordLoginReq{}
	if err := ctx.BindJSON(req); err != nil {
		api.WriteResp(ctx, nil, common.ParamErr.WithErr(err))
		return
	}
	resp, errno := c.user.MobilePasswordLogin(ctx.Request.Context(), req)
	api.WriteResp(ctx, resp, errno)
}

func (c *Ctrl) MobilePasswordReset(ctx *gin.Context) {
	req := &dto.MobilePasswordResetReq{}
	if err := ctx.BindJSON(req); err != nil {
		api.WriteResp(ctx, nil, common.ParamErr.WithErr(err))
		return
	}
	errno := c.user.MobilePasswordReset(ctx.Request.Context(), req)
	api.WriteResp(ctx, nil, errno)
}

func (c *Ctrl) MobileVerifyLogin(ctx *gin.Context) {
	req := &dto.MobileVerifyCodeLoginReq{}
	if err := ctx.BindJSON(req); err != nil {
		api.WriteResp(ctx, nil, common.ParamErr.WithErr(err))
		return
	}
	resp, errno := c.user.MobileVerifyLogin(ctx.Request.Context(), req)
	api.WriteResp(ctx, resp, errno)
}
