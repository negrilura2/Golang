package customer

import (
	"github.com/gin-gonic/gin"
	"mall/api"
	"mall/common"
	"mall/consts"
	"mall/service/dto"
)

func (c *Ctrl) GetCourseList(ctx *gin.Context) {
	user := api.GetUserFromCtx(ctx)
	req := &dto.CourseListReq{}
	if err := ctx.BindQuery(req); err != nil {
		api.WriteResp(ctx, nil, common.ParamErr.WithErr(err))
		return
	}
	req.UserID = user.User.ID
	req.Status = consts.IsEnable
	resp, errno := c.course.ListCourse(ctx.Request.Context(), req)
	api.WriteResp(ctx, resp, errno)
}

func (c *Ctrl) GetCourseDetail(ctx *gin.Context) {
	req := &dto.CourseInfoReq{}
	if err := ctx.BindQuery(req); err != nil {
		api.WriteResp(ctx, nil, common.ParamErr.WithErr(err))
		return
	}
	resp, errno := c.course.GetCourseDetail(ctx.Request.Context(), req)

	api.WriteResp(ctx, resp, errno)
}

func (c *Ctrl) GetCourseLessonInfo(ctx *gin.Context) {
	user := api.GetUserFromCtx(ctx)
	req := &dto.LessonInfoReq{}
	if err := ctx.BindQuery(req); err != nil {
		api.WriteResp(ctx, nil, common.ParamErr.WithErr(err))
		return
	}
	req.UserId = user.User.ID
	resp, isAes, errno := c.course.LessonDetail(ctx.Request.Context(), req)
	if isAes {
		api.WriteRespAes(ctx, resp, errno)
		return
	}
	api.WriteResp(ctx, resp, errno)
}

func (c *Ctrl) GetPurchasedCourseList(ctx *gin.Context) {
	user := api.GetUserFromCtx(ctx)
	req := &dto.GetPurchasedCourseReq{}
	if err := ctx.BindQuery(req); err != nil {
		api.WriteResp(ctx, nil, common.ParamErr.WithErr(err))
		return
	}
	resp, errno := c.course.GetPurchasedCourseList(ctx.Request.Context(), user, req)
	api.WriteResp(ctx, resp, errno)
}
