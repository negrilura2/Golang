package user

import (
	"context"
	"github.com/go-redis/redis"
	"github.com/gogf/gf/util/gconv"
	"github.com/samber/lo"
	"gorm.io/gorm"
	"mall/adaptor"
	"mall/adaptor/repo/model"
	"mall/adaptor/repo/query"
	"mall/consts"
	"mall/service/do"
	"mall/utils/tools"
	"time"
)

type IUser interface {
	MobileCreateUser(ctx context.Context, req *do.MobileCreateUser) (*model.MobileUser, error)
	WechatCreateUser(ctx context.Context, req *do.WechatCreateUser) (int64, error)
	CreateUserByWechatApp(ctx context.Context, req *do.CreateUserByWechatApp) (*model.User, error)
	UpdateUserPassword(ctx context.Context, req *do.UpdateUserPassword) error

	GetMobileUsersByUserIds(ctx context.Context, userIds []int64) ([]*model.MobileUser, error)
	GetMobileUserByUserID(ctx context.Context, userId int64) (*model.MobileUser, error)
	GetUserByMobile(ctx context.Context, mobileSha256 string) (*model.MobileUser, error)
	GetUserByID(ctx context.Context, userId int64) (*model.User, error)
	GetWechatUserByUserID(ctx context.Context, userId int64) (*model.WechatUser, error)
	GetAppUsersByUserID(ctx context.Context, userId int64) ([]*model.AppUser, error)
	GetUserNameMap(ctx context.Context, ids []int64) (map[int64]string, error)

	GetWechatAppUser(ctx context.Context, appCode int32, openID string) (*model.AppUser, error)

	ListUser(ctx context.Context, req *do.ListUser) ([]*model.User, int64, error)
}

type User struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewUser(adaptor adaptor.IAdaptor) *User {
	return &User{
		db:    adaptor.GetDB(),
		redis: adaptor.GetRedis(),
	}
}
func (a *User) MobileCreateUser(ctx context.Context, req *do.MobileCreateUser) (*model.MobileUser, error) {
	timeNow := time.Now()
	mobileUser := &model.MobileUser{
		MobileAes:    req.MobileAes,
		MobileSha256: req.MobileSha256,
		CreateAt:     timeNow,
		UpdateAt:     timeNow,
	}
	err := a.db.Transaction(func(tx *gorm.DB) error {
		// 创建用户基本信息
		user := &model.User{
			NickName:    req.NickName,
			Sex:         req.Sex,
			Password:    "",
			Status:      consts.IsEnable,
			IconKey:     "",
			CreateAt:    timeNow,
			LastLoginAt: timeNow,
			UpdateAt:    timeNow,
		}

		if err := tx.WithContext(ctx).Create(user).Error; err != nil {
			return err
		}
		// 创建手机号用户关联记录
		mobileUser.UserID = user.ID
		if err := tx.WithContext(ctx).Create(mobileUser).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return mobileUser, nil
}
func (a *User) WechatCreateUser(ctx context.Context, req *do.WechatCreateUser) (int64, error) {
	return 0, nil
}

func (a *User) CreateUserByWechatApp(ctx context.Context, req *do.CreateUserByWechatApp) (*model.User, error) {
	timeNow := time.Now()
	user := &model.User{
		NickName:    gconv.String(req.AppCode) + req.OpenID[:5],
		Sex:         consts.SexUnknown,
		Password:    "",
		Status:      consts.IsEnable,
		IconKey:     "",
		CreateAt:    timeNow,
		LastLoginAt: timeNow,
		UpdateAt:    timeNow,
	}
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&model.User{}).Create(user).Error
		if err != nil {
			return err
		}
		appUser := &model.AppUser{
			UserID:   user.ID,
			AppCode:  req.AppCode,
			OpenID:   req.OpenID,
			Status:   consts.IsEnable,
			CreateAt: timeNow,
			UpdateAt: timeNow,
		}
		return tx.Model(&model.AppUser{}).Create(appUser).Error
	})
	return user, err
}

func (a *User) UpdateUserPassword(ctx context.Context, req *do.UpdateUserPassword) error {
	qs := query.Use(a.db).User
	_, err := qs.WithContext(ctx).Where(qs.ID.Eq(req.UserID)).Updates(model.User{
		Password: req.NewPassword,
	})
	return err
}

func (a *User) GetUserByMobile(ctx context.Context, mobileSha256 string) (*model.MobileUser, error) {
	qs := query.Use(a.db).MobileUser
	return qs.WithContext(ctx).Where(qs.MobileSha256.Eq(mobileSha256)).First()
}

func (a *User) GetMobileUserByUserID(ctx context.Context, userId int64) (*model.MobileUser, error) {
	qs := query.Use(a.db).MobileUser
	return qs.WithContext(ctx).Where(qs.UserID.Eq(userId)).First()
}

func (a *User) GetMobileUsersByUserIds(ctx context.Context, userIds []int64) ([]*model.MobileUser, error) {
	qs := query.Use(a.db).MobileUser
	return qs.WithContext(ctx).Where(qs.UserID.In(userIds...)).Find()
}

func (a *User) GetUserByID(ctx context.Context, userId int64) (*model.User, error) {
	qs := query.Use(a.db).User
	return qs.WithContext(ctx).Where(qs.ID.Eq(userId)).First()
}

func (a *User) GetAppUsersByUserID(ctx context.Context, userId int64) ([]*model.AppUser, error) {
	qs := query.Use(a.db).AppUser
	return qs.WithContext(ctx).Where(qs.UserID.Eq(userId)).Find()
}

func (a *User) GetWechatUserByUserID(ctx context.Context, userId int64) (*model.WechatUser, error) {
	qs := query.Use(a.db).WechatUser
	return qs.WithContext(ctx).Where(qs.UserID.Eq(userId)).First()
}

func (a *User) GetUserNameMap(ctx context.Context, ids []int64) (map[int64]string, error) {
	qs := query.Use(a.db).User
	list, err := qs.WithContext(ctx).Where(qs.ID.In(ids...)).Find()
	if err != nil {
		return nil, err
	}
	return lo.SliceToMap(list, func(item *model.User) (int64, string) {
		return item.ID, item.NickName
	}), err
}

func (a *User) ListUser(ctx context.Context, req *do.ListUser) ([]*model.User, int64, error) {
	qs := query.Use(a.db).User
	mqs := query.Use(a.db).MobileUser
	tx := qs.WithContext(ctx)
	if len(req.UserIds) > 0 {
		tx = tx.Where(qs.ID.In(req.UserIds...))
	}
	if req.MobileSha256 != "" {
		tx = tx.Join(mqs, mqs.UserID.EqCol(qs.ID)).Where(mqs.MobileSha256.Eq(req.MobileSha256))
	}
	if req.Status != 0 {
		tx = tx.Where(qs.Status.Eq(req.Status))
	}
	if req.NickNameKw != "" {
		tx = tx.Where(qs.NickName.Like(tools.GetAllLike(req.NickNameKw)))
	}
	return tx.Order(qs.LastLoginAt.Desc(), qs.ID.Desc()).FindByPage(req.GetOffset(), req.Limit)
}

func (a *User) GetWechatAppUser(ctx context.Context, appCode int32, openID string) (*model.AppUser, error) {
	qs := query.Use(a.db).AppUser
	return qs.WithContext(ctx).Where(qs.AppCode.Eq(appCode), qs.OpenID.Eq(openID)).First()
}
