package user

import (
	"context"
	"github.com/samber/lo"
	"gorm.io/gorm"
	"mall/adaptor"
	"mall/adaptor/repo/model"
	"mall/adaptor/repo/query"
	"mall/service/do"
	"time"
)

type IUserCourse interface {
	ListUserCourse(ctx context.Context, req *do.ListUserCourse) ([]*model.UserCourseGood, int64, error)
	CreateUserCourse(ctx context.Context, tx *gorm.DB, req *do.CreateUserCourse) error
	DeleteUserCourse(ctx context.Context, tx *gorm.DB, req *do.DeleteUserCourse) error
}

type UserCourse struct {
	db *gorm.DB
}

func NewUserCourse(adaptor adaptor.IAdaptor) *UserCourse {
	return &UserCourse{
		db: adaptor.GetDB(),
	}
}
func (s *UserCourse) ListUserCourse(ctx context.Context, req *do.ListUserCourse) ([]*model.UserCourseGood, int64, error) {
	qs := query.Use(s.db).UserCourseGood
	tx := qs.WithContext(ctx).Where(qs.UserID.Eq(req.UserId))
	if req.Valid {
		// 可看时间大于当前时间为有效
		tx = tx.Where(qs.LearnExpireTime.Gte(time.Now().UnixMilli()))
	}
	list, count, err := tx.Order(qs.BuyTime.Desc()).
		FindByPage(req.GetOffset(), req.Limit)
	if err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

func (s *UserCourse) CreateUserCourse(ctx context.Context, tx *gorm.DB, req *do.CreateUserCourse) error {
	if req.CourseList == nil || len(req.CourseList) == 0 {
		return nil
	}
	qs := query.Use(tx).UserCourseGood
	addList := make([]*model.UserCourseGood, 0)
	lo.ForEach(req.CourseList, func(item do.BuyCourseGoods, index int) {
		addList = append(addList, &model.UserCourseGood{
			UserID:            req.UserId,
			OrderID:           req.OrderID,
			OrderItemID:       item.OrderItemID,
			GoodsType:         item.GoodType,
			GoodsID:           item.GoodsID,
			BuyTime:           req.BuyTime,
			LearnExpireTime:   item.LearnExpireTime,
			ServiceExpireTime: item.ServiceExpireTime,
		})
	})
	return qs.WithContext(ctx).CreateInBatches(addList, 100)
}

func (s *UserCourse) DeleteUserCourse(ctx context.Context, tx *gorm.DB, req *do.DeleteUserCourse) error {
	qs := query.Use(tx).UserCourseGood
	_, err := qs.WithContext(ctx).
		Where(qs.UserID.Eq(req.UserID),
			qs.OrderID.Eq(req.OrderID),
			qs.OrderItemID.In(req.OrderItemIds...)).
		Delete()
	return err
}
