package router

import (
	"github.com/gin-gonic/gin"
	"mall/consts"
	"mall/utils/logger"
	"mall/utils/tools"
)

func RequestIDMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		rid := ctx.GetHeader(consts.RequestIDKey)
		if rid == "" {
			rid = tools.UUIDHex()
		}
		ctx.Header(consts.RequestIDKey, rid)
		ctx.Set(consts.RequestIDKey, rid)
		ctx.Request = ctx.Request.WithContext(
			logger.WithRequestID(ctx.Request.Context(), rid))
		ctx.Next()
	}
}
