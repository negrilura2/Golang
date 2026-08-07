package token

import (
	"context"
	"go.uber.org/zap"
	"mall/common"
	"mall/utils/logger"
)

func (s *Service) GetWechatAppletAccessToken(ctx context.Context, appCode int32, force bool) (*AccessToken, common.Errno) {
	token, err := s.getWechatAppletAccessToken(ctx, appCode, force)
	if err != nil {
		logger.Error("GetWechatAppletAccessToken get access token failed", zap.Error(err), zap.Any("app_code", appCode), zap.Any("force", force))
		return nil, common.ServerErr.WithErr(err)
	}
	return token, common.OK
}

func (s *Service) getWechatAppletAccessToken(ctx context.Context, appCode int32, force bool) (*AccessToken, error) {
	getTokenFunc := func() (*AccessToken, error) {
		token, err := s.wechat.GetAppletAccessToken(ctx, appCode)
		if err != nil {
			logger.Error("getWechatAppletAccessToken GetAppletAccessToken get access token failed", zap.Error(err), zap.Any("app_code", appCode), zap.Any("force", force))
			return nil, common.ServerErr.WithErr(err)
		}
		return &AccessToken{
			Token:     token.AccessToken,
			ExpiresIn: token.ExpiresIn,
		}, nil
	}
	lockKey := s.lockTokenKeyFmt(appCode)
	cacheKey := s.cacheTokenKeyFmt(appCode)

	token, err := s.getCache(ctx, cacheKey)
	if err != nil {
		logger.Error("getWechatAppletAccessToken getCache failed",
			zap.Error(err),
			zap.Any("app_code", appCode),
			zap.Any("force", force))
	}
	// 缓存没有获取到，或者调用方强刷时，进行调用获取
	if token == nil || force {
		return s.updateToken(ctx, getTokenFunc, lockKey, cacheKey)
	}
	// 缓存获取成功
	return token, nil
}
