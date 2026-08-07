package user

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"mall/adaptor"
	"mall/adaptor/repo/model"
	"mall/adaptor/repo/query"
	"mall/service/do"
	"mall/utils/tools"
	"time"
)

type IUserCart interface {
	AddGoods(ctx context.Context, req *do.AddGoods) (int64, error)
	RemoveGoods(ctx context.Context, req *do.RemoveGoods) error
	ListGoods(ctx context.Context, req *do.ListGoods) ([]*model.UserCart, int64, error)
}

type UserCart struct {
	db *gorm.DB
}

func NewUserCart(adaptor adaptor.IAdaptor) *UserCart {
	return &UserCart{
		db: adaptor.GetDB(),
	}
}

func (s *UserCart) AddGoods(ctx context.Context, req *do.AddGoods) (int64, error) {
	qs := query.Use(s.db).UserCart
	addObj := &model.UserCart{
		GoodsID:  req.GoodsID,
		UserID:   req.UserID,
		Quantity: 1,
		AddAt:    time.Now(),
	}
	err := s.db.WithContext(ctx).Create(addObj).Error
	if err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
		return 0, err
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		get, err := qs.WithContext(ctx).Where(qs.UserID.Eq(req.UserID), qs.GoodsID.Eq(req.GoodsID)).First()
		if err != nil || get == nil {
			return 0, err
		}
		return get.ID, nil
	}
	return addObj.ID, nil
}
func (s *UserCart) RemoveGoods(ctx context.Context, req *do.RemoveGoods) error {
	qs := query.Use(s.db).UserCart
	_, err := qs.WithContext(ctx).Where(qs.UserID.Eq(req.UserID), qs.ID.Eq(req.ID)).Delete()
	return err
}
func (s *UserCart) ListGoods(ctx context.Context, req *do.ListGoods) ([]*model.UserCart, int64, error) {
	qs := query.Use(s.db).UserCart
	cqs := query.Use(s.db).CourseGood
	tx := qs.WithContext(ctx).Where(qs.UserID.Eq(req.UserID))
	if req.GoodsNameKW != "" {
		tx = tx.Join(cqs, cqs.ID.EqCol(qs.GoodsID)).Where(cqs.Name.Like(tools.GetAllLike(req.GoodsNameKW)))
	}
	return tx.Order(qs.ID.Desc()).FindByPage(req.GetOffset(), req.Limit)
}
