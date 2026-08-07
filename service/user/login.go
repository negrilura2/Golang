package user

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/gogf/gf/util/gconv"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"mall/adaptor/repo/model"
	"mall/adaptor/rpc"
	"mall/common"
	"mall/consts"
	"mall/service/do"
	"mall/service/dto"
	"mall/utils/logger"
	"mall/utils/pool"
	"mall/utils/tools"
	"time"
)

func (s *Service) GetSmsVerifyCode(ctx context.Context, req *dto.GetSmsVerifyCodeReq) common.Errno {
	_, err := s.verify.GetCaptchaTicket(ctx, req.Ticket)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return common.InvalidCaptchaErr
		}
		logger.Error("GetSmsVerifyCode GetCaptchaTicket error", zap.Error(err), zap.String("mobile", req.Mobile))
		return common.RedisErr.WithErr(err)
	}
	// 校验手机号是否注册
	if req.Scene == consts.RegisterUserSmsCode || req.Scene == consts.UserReSetPasswordSmsCode {
		mobileSha256 := tools.Sha256Hash(req.Mobile)
		mobileUser, err := s.user.GetUserByMobile(ctx, mobileSha256)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Error("GetSmsVerifyCode GetUserByMobile error", zap.Error(err), zap.String("mobile", req.Mobile))
			return common.DatabaseErr.WithErr(err)
		}
		if mobileUser != nil {
			return common.MobileRegisteredErr
		}
		if req.Scene == consts.RegisterUserSmsCode {
			// 注册即登录
			req.Scene = consts.UserMobileLoginSmsCode
		}
	}
	verifyCode := tools.GenValidateCode(4)
	tokenFun := func(ctx context.Context, force bool) (string, error) {
		token, errno := s.token.GetLarkTenantAccessToken(ctx, consts.LarkAppCode, force)
		if errno.NotOk() {
			return "", common.ServerErr.WithErr(errno)
		}
		return token.Token, nil
	}
	err = s.lark.SendLarkMsg(ctx, tokenFun, &do.SendLarkMsg{
		AppCode: consts.LarkAppCode,
		OpenID:  s.conf.BizConf.LarkGroupID,
		IDType:  rpc.LarkChatGroupType,
		Content: fmt.Sprintf("<b>手机验证码通知</b>\\n\\n手机号：%s \\n验证码：%s", req.Mobile, verifyCode),
	})
	if err != nil {
		logger.Error("GetSmsVerifyCode SendLarkMsg error", zap.Error(err), zap.String("mobile", req.Mobile))
		return common.ServerErr.WithErr(err)
	}
	err = s.verify.SetVerifyCode(ctx, req.Mobile, req.Scene, verifyCode, 5*time.Minute)
	if err != nil {
		logger.Error("GetSmsVerifyCode SetVerifyCode error", zap.Error(err), zap.String("mobile", req.Mobile))
		return common.RedisErr.WithErr(err)
	}
	return common.OK
}

func (s *Service) checkSmsVerifyCode(ctx context.Context, mobile, scene, verifyCode string) bool {
	getCode, err := s.verify.GetVerifyCode(ctx, mobile, scene)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false
		}
		logger.Error("CheckSmsVerifyCode GetVerifyCode error", zap.Error(err), zap.String("mobile", mobile))
	}
	if getCode != verifyCode {
		return false
	}
	s.verify.DelVerifyCode(ctx, mobile, scene)
	return true
}

func (s *Service) MobileVerifyLogin(ctx context.Context, req *dto.MobileVerifyCodeLoginReq) (*dto.LoginResp, common.Errno) {
	pass := s.checkSmsVerifyCode(ctx, req.Mobile, consts.UserMobileLoginSmsCode, req.VerifyCode)
	if !pass {
		return nil, common.InvalidSmsCodeErr
	}
	mobileSha256 := tools.Sha256Hash(req.Mobile)
	mobileUser, err := s.user.GetUserByMobile(ctx, mobileSha256)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("MobileVerifyLogin GetUserByMobile error", zap.Error(err), zap.String("mobile", req.Mobile))
		return nil, common.DatabaseErr.WithErr(err)
	}
	// 如果手机号用户不存在，则进行通过手机号创建一个用户
	if mobileUser == nil {
		mobileAes, err := tools.AESEncrypt(req.Mobile, []byte(s.conf.BizConf.MobileSecret))
		if err != nil {
			logger.Error("MobileVerifyLogin AESEncrypt error", zap.Error(err), zap.String("mobile", req.Mobile))
			return nil, common.ServerErr.WithErr(err)
		}
		mobileUser, err = s.user.MobileCreateUser(ctx, &do.MobileCreateUser{
			MobileAes:    mobileAes,    // 密文-用于回显
			MobileSha256: mobileSha256, // 密文-用于查询
			NickName:     fmt.Sprintf("手机号用户%s", req.Mobile[len(req.Mobile)-4:]),
			Sex:          consts.SexUnknown,
		})
		if err != nil {
			logger.Error("MobileVerifyLogin MobileCreateUser error", zap.Error(err), zap.String("mobile", req.Mobile))
			return nil, common.DatabaseErr.WithErr(err)
		}
	}
	user, err := s.user.GetUserByID(ctx, mobileUser.UserID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("MobileVerifyLogin GetUserByID error", zap.Error(err), zap.String("mobile", req.Mobile))
		return nil, common.DatabaseErr.WithErr(err)
	}
	if user == nil || user.Status != consts.IsEnable {
		return nil, common.UserNotFoundErr
	}
	userInfo, err := s.packageUserInfo(ctx, user)
	if err != nil {
		logger.Error("MobileVerifyLogin packageUserInfo error", zap.Error(err), zap.String("mobile", req.Mobile))
		return nil, common.DatabaseErr.WithErr(err)
	}
	return s.handleUserLogin(ctx, userInfo)
}

func (s *Service) packageUserInfo(ctx context.Context, user *model.User) (*dto.UserInfoDto, error) {
	var (
		mobileUser *model.MobileUser
		appUsers   []*model.AppUser
		wechatUser *model.WechatUser
	)
	tempPool := pool.NewPoolWithSize(3)
	defer tempPool.Release()
	tempPool.RunGo(func() {
		tempUser, err := s.user.GetMobileUserByUserID(ctx, user.ID)
		if err != nil {
			logger.Error("MobileVerifyLogin GetMobileUserByUserID error", zap.Error(err), zap.Int64("user_id", user.ID))
			return
		}
		mobileUser = tempUser
	})
	tempPool.RunGo(func() {
		tempAppUsers, err := s.user.GetAppUsersByUserID(ctx, user.ID)
		if err != nil {
			logger.Error("MobileVerifyLogin GetAppUsersByUserID error", zap.Error(err), zap.Int64("user_id", user.ID))
			return
		}
		appUsers = tempAppUsers
	})
	tempPool.RunGo(func() {
		tempWechatUser, err := s.user.GetWechatUserByUserID(ctx, user.ID)
		if err != nil {
			logger.Error("MobileVerifyLogin GetWechatUserByUserID error", zap.Error(err), zap.Int64("user_id", user.ID))
			return
		}
		wechatUser = tempWechatUser
	})
	tempPool.Wait()
	retUserInfo := &dto.UserInfoDto{
		User: dto.UserDto{
			ID:          user.ID,
			NickName:    user.NickName,
			CreateAt:    user.CreateAt.UnixMilli(),
			IconUrl:     user.IconKey,
			Sex:         user.Sex,
			Status:      user.Status,
			LastLoginAt: user.LastLoginAt.UnixMilli(),
			UpdateAt:    user.UpdateAt.UnixMilli(),
		},
	}
	if mobileUser != nil {
		mobile, err := tools.AESDecrypt(mobileUser.MobileAes, []byte(s.conf.BizConf.MobileSecret))
		if err != nil {
			logger.Error("MobileVerifyLogin AESDecrypt error", zap.Error(err), zap.String("mobile", mobileUser.MobileAes))
			return nil, common.ServerErr.WithErr(err)
		}
		retUserInfo.MobileUser = &dto.MobileUser{
			Mobile: string(mobile),
			UserID: mobileUser.UserID,
		}
	}
	if wechatUser != nil {
		retUserInfo.WechatUser = &dto.WechatUser{
			UserID:  wechatUser.UserID,
			UnionID: wechatUser.UnionID,
		}
	}
	for _, app := range appUsers {
		retUserInfo.AppUsers = append(retUserInfo.AppUsers, dto.AppUser{
			AppCode: app.AppCode,
			OpenID:  app.OpenID,
			UserID:  app.UserID,
		})
	}
	return retUserInfo, nil
}

func (s *Service) handleUserLogin(ctx context.Context, userInfo *dto.UserInfoDto) (*dto.LoginResp, common.Errno) {
	tokenUuid := tools.UUIDHex()
	// 处理token
	err := s.processToken(ctx, tokenUuid, userInfo)
	if err != nil {
		logger.Error("MobileVerifyLogin processToken error", zap.Error(err))
		return nil, common.RedisErr.WithErr(err)
	}
	return &dto.LoginResp{
		Token:    tokenUuid,
		UserInfo: userInfo,
	}, common.OK
}

func (s *Service) processToken(ctx context.Context, token string, user *dto.UserInfoDto) error {
	err := s.verify.SetUserToken(ctx, user.User.ID, token, gconv.String(user), consts.CustomerUserTokenExpire)
	if err != nil {
		logger.Error("SetAdminUserToken error", zap.Error(err), zap.Any("user", user))
		return err
	}
	return nil
}

// 小程序登录
func (s *Service) AppletLogin(ctx context.Context, req *dto.AppletLoginReq) (*dto.LoginResp, common.Errno) {
	resp, err := s.wechat.Code2Session(ctx, req.AppCode, req.Code)
	if err != nil {
		logger.Error("Code2Session error", zap.Error(err), zap.Int32("app_code", req.AppCode))
		return nil, common.ParamErr.WithErr(err)
	}
	wechatAppUser, err := s.user.GetWechatAppUser(ctx, req.AppCode, resp.Openid)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("AppletLogin GetWechatAppUser error", zap.Error(err), zap.Int32("app_code", req.AppCode))
		return nil, common.DatabaseErr.WithErr(err)
	}

	var (
		user *model.User
	)

	if wechatAppUser != nil { // 存在则获取用户信息
		user, err = s.user.GetUserByID(ctx, wechatAppUser.UserID)
		if err != nil {
			logger.Error("AppletLogin GetUserByID error", zap.Error(err), zap.Int32("app_code", req.AppCode))
			return nil, common.DatabaseErr.WithErr(err)
		}
	} else { // 不存在则创建用户
		user, err = s.user.CreateUserByWechatApp(ctx, &do.CreateUserByWechatApp{
			OpenID:  resp.Openid,
			AppCode: req.AppCode,
		})
		if err != nil {
			logger.Error("AppletLogin CreateUserByWechatApp error", zap.Error(err), zap.Int32("app_code", req.AppCode))
			return nil, common.DatabaseErr.WithErr(err)
		}
	}
	userInfo, err := s.packageUserInfo(ctx, user)
	if err != nil {
		logger.Error("AppletLogin packageUserInfo error", zap.Error(err), zap.Int32("app_code", req.AppCode))
		return nil, common.DatabaseErr.WithErr(err)
	}
	return s.handleUserLogin(ctx, userInfo)
}

func (s *Service) MobilePasswordLogin(ctx context.Context, req *dto.MobilePasswordLoginReq) (*dto.LoginResp, common.Errno) {
	_, err := s.verify.GetCaptchaTicket(ctx, req.Ticket)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, common.InvalidCaptchaErr
		}
		logger.Error("MobileLogin GetCaptchaTicket error", zap.Error(err), zap.String("mobile", req.Mobile))
		return nil, common.RedisErr.WithErr(err)
	}
	mobileSha256 := tools.Sha256Hash(req.Mobile)
	mobileUser, err := s.user.GetUserByMobile(ctx, mobileSha256)
	if err != nil {
		logger.Error("MobilePasswordLogin GetUserByMobile error", zap.Error(err), zap.String("mobile", req.Mobile))
		return nil, common.DatabaseErr.WithErr(err)
	}
	user, err := s.user.GetUserByID(ctx, mobileUser.UserID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("MobilePasswordLogin GetUserByID error", zap.Error(err), zap.String("mobile", req.Mobile))
		return nil, common.DatabaseErr.WithErr(err)
	}
	if user == nil || user.Status != consts.IsEnable {
		return nil, common.UserNotFoundErr
	}
	// 进行用户密码校验累计
	errCount, err := s.verify.IncrPasswordErr(ctx, req.Mobile, consts.PasswordErrExpire)
	if err != nil {
		logger.Error("MobilePasswordLogin IncrPasswordErr error", zap.Error(err), zap.String("mobile", req.Mobile))
		return nil, common.RedisErr.WithErr(err)
	}
	if errCount > consts.PasswordErrMaxCount {
		return nil, common.PasswordErrLimit
	}
	if user.Password != req.Password {
		return nil, common.InvalidPasswordErr
	}
	_ = s.verify.DeletePasswordErr(ctx, req.Mobile)

	userInfo, err := s.packageUserInfo(ctx, user)
	if err != nil {
		logger.Error("MobilePasswordLogin packageUserInfo error", zap.Error(err), zap.String("mobile", req.Mobile))
		return nil, common.DatabaseErr.WithErr(err)
	}
	return s.handleUserLogin(ctx, userInfo)
}

func (s *Service) MobilePasswordReset(ctx context.Context, req *dto.MobilePasswordResetReq) common.Errno {
	pass := s.checkSmsVerifyCode(ctx, req.Mobile, consts.UserReSetPasswordSmsCode, req.VerifyCode)
	if !pass {
		return common.InvalidSmsCodeErr
	}
	if req.Password != req.ConfirmPassword {
		return common.ConfirmPasswordErr
	}
	mobileSha256 := tools.Sha256Hash(req.Mobile)
	mobileUser, err := s.user.GetUserByMobile(ctx, mobileSha256)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return common.UserNotFoundErr
		}
		logger.Error("MobilePasswordReset GetUserByMobile error", zap.Error(err), zap.String("mobile", req.Mobile))
		return common.DatabaseErr.WithErr(err)
	}
	user, err := s.user.GetUserByID(ctx, mobileUser.UserID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("MobilePasswordReset GetUserByID error", zap.Error(err), zap.String("mobile", req.Mobile))
		return common.DatabaseErr.WithErr(err)
	}
	if user == nil || user.Status != consts.IsEnable {
		return common.UserNotFoundErr
	}

	err = s.user.UpdateUserPassword(ctx, &do.UpdateUserPassword{
		UserID:      user.ID,
		NewPassword: req.ConfirmPassword,
	})
	if err != nil {
		logger.Error("MobilePasswordReset UpdateUserPassword error", zap.Error(err), zap.String("mobile", req.Mobile))
		return common.DatabaseErr.WithErr(err)
	}
	_ = s.verify.CleanUserToken(ctx, user.ID)
	return common.OK
}
