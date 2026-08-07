package user

import (
	"context"
	"encoding/json"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"mall/adaptor/repo/model"
	"mall/common"
	"mall/service/do"
	"mall/service/dto"
	"mall/utils/logger"
	"mall/utils/tools"
)

func (s *Service) GetUserByToken(ctx context.Context, token string) (*common.UserInfo, error) {
	valueStr, err := s.verify.GetUserToken(ctx, token)
	if err != nil {
		logger.Error("GetUserByToken GetUserToken", zap.Error(err), zap.String("token", token))
		return nil, common.DatabaseErr.WithErr(err)
	}
	userInfo := &common.UserInfo{}
	json.Unmarshal([]byte(valueStr), userInfo)
	return userInfo, nil
}

func (s *Service) CustomerUserList(ctx context.Context, req *dto.ListCustomerUserReq) (*dto.ListCustomerUserResp, common.Errno) {
	var (
		userIds      = []int64{}
		mobileSha256 string
		fileKeys     = []string{}
		fileUrlMap   = make(map[string]string)
	)
	if req.ID > 0 {
		userIds = append(userIds, req.ID)
	}
	if req.Mobile != "" {
		mobileSha256 = tools.Sha256Hash(req.Mobile)
	}
	list, count, err := s.user.ListUser(ctx, &do.ListUser{
		Pager:        req.Pager,
		UserIds:      userIds,
		NickNameKw:   req.NickNameKw,
		MobileSha256: mobileSha256,
		Status:       req.Status,
	})
	if err != nil {
		logger.Error("CustomerUserList ListUser", zap.Error(err))
		return nil, common.DatabaseErr.WithErr(err)
	}
	lo.ForEach(list, func(item *model.User, index int) {
		userIds = append(userIds, item.ID)
		fileKeys = append(fileKeys, item.IconKey)
	})
	fileUrlMap, err = s.storage.GetPreviewUrl(ctx, &do.GetPreviewUrl{
		Keys:        fileKeys,
		ExpireHours: 2,
	})
	if err != nil {
		logger.Error("CustomerUserList GetPreviewUrl", zap.Error(err))
		return nil, common.ServerErr.WithErr(err)
	}
	mobileUserList, err := s.user.GetMobileUsersByUserIds(ctx, userIds)
	if err != nil {
		logger.Error("CustomerUserList GetMobileUsersByUserIds", zap.Error(err))
		return nil, common.DatabaseErr.WithErr(err)
	}
	mobileUserMap := lo.SliceToMap(mobileUserList, func(item *model.MobileUser) (int64, *model.MobileUser) {
		return item.UserID, item
	})

	retList := make([]*dto.CustomerUserDto, 0)
	lo.ForEach(list, func(item *model.User, index int) {
		dtoUser := dto.UserDto{
			ID:          item.ID,
			NickName:    item.NickName,
			Sex:         item.Sex,
			IconUrl:     fileUrlMap[item.IconKey],
			Status:      item.Status,
			CreateAt:    item.CreateAt.UnixMilli(),
			LastLoginAt: item.LastLoginAt.UnixMilli(),
			UpdateAt:    item.UpdateAt.UnixMilli(),
		}
		customerUser := &dto.CustomerUserDto{
			UserDto: dtoUser,
		}
		mobileUser, ok := mobileUserMap[item.ID]
		if ok {
			mobile, err := tools.AESDecrypt(mobileUser.MobileAes, []byte(s.conf.BizConf.MobileSecret))
			if err != nil {
				logger.Error("CustomerUserList AESDecrypt", zap.Error(err), zap.String("mobile_aes", mobileUser.MobileAes))
			} else {
				customerUser.Mobile = string(mobile)
			}
		}
		retList = append(retList, customerUser)
	})
	return &dto.ListCustomerUserResp{
		List:  retList,
		Pager: req.Pager,
		Total: count,
	}, common.OK
}

func (s *Service) CustomerUserInfo(ctx context.Context, req *dto.GetCustomerUserInfoReq) (*dto.CustomerUserInfoDto, common.Errno) {
	dtoUserInfo, errno := s.GetUserInfo(ctx, &common.UserInfo{
		User: common.User{
			ID: req.ID,
		},
	})
	if errno.NotOk() {
		logger.Error("CustomerUserInfo GetUserInfo", zap.Error(errno), zap.Any("user_id", req.ID))
		return nil, errno
	}
	resp, errno := s.goodsSvc.GetPurchasedCourseList(ctx, &common.UserInfo{
		User: common.User{
			ID: req.ID,
		},
	}, &dto.GetPurchasedCourseReq{
		Pager: common.Pager{
			Page:  1,
			Limit: 20,
		},
	})
	if errno.NotOk() {
		logger.Error("CustomerUserInfo GetPurchasedCourseList", zap.Error(errno), zap.Any("user_id", req.ID))
		return nil, errno
	}

	return &dto.CustomerUserInfoDto{
		UserInfoDto: dtoUserInfo,
		UserCourses: resp.List,
	}, common.OK
}

func (s *Service) GetUserInfo(ctx context.Context, authUser *common.UserInfo) (*dto.UserInfoDto, common.Errno) {
	user, err := s.user.GetUserByID(ctx, authUser.User.ID)
	if err != nil {
		logger.Error("GetUserInfo GetUserByID", zap.Error(err), zap.Int64("user_id", authUser.User.ID))
		return nil, common.DatabaseErr.WithErr(err)
	}
	userInfo, err := s.packageUserInfo(ctx, user)
	if err != nil {
		logger.Error("GetUserInfo packageUserInfo", zap.Error(err), zap.Int64("user_id", authUser.User.ID))
		return nil, common.DatabaseErr.WithErr(err)
	}
	return userInfo, common.OK
}
