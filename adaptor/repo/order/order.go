package order

import (
	"context"
	"fmt"
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

type IOrder interface {
	CreateOrder(ctx context.Context, req *do.CreateOrder) error
	GetOrderByID(ctx context.Context, orderID int64) (*model.Order, error)
	GetOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*model.Order, error)
	GetOrderItems(ctx context.Context, orderID int64) ([]*model.OrderItem, error)
	GetOrderItemByOrderIDs(ctx context.Context, orderIds []int64) (map[int64][]*model.OrderItem, error)
	GetOrderRefunds(ctx context.Context, orderID int64) ([]*model.OrderRefund, error)
	GetOrderList(ctx context.Context, req *do.GetOrderList) ([]*model.Order, int64, error)
	CancelOrder(ctx context.Context, req *do.CancelOrder) error
	UpdateOrderPaySuccess(ctx context.Context, req *do.UpdateOrderPaySuccess) error
	UpdateOrderOutTradeNo(ctx context.Context, orderID int64, outTradeNo string) error
	// 订单统计
	OrderStatistic(ctx context.Context, req *do.OrderStat) ([]*do.OrderStatistic, int64, error)
	// 订单退款
	OrderRefund(ctx context.Context, req *do.OrderRefund) (int64, error)
	GetOrderRefundByOrderID(ctx context.Context, orderID int64) (*model.OrderRefund, error)
	GetOrderRefundByOutTradeNo(ctx context.Context, outTradeNo string) (*model.OrderRefund, error)
	OrderRefundResult(ctx context.Context, req *do.OrderRefundResult) error
}

type Order struct {
	db *gorm.DB
}

func NewOrder(adaptor adaptor.IAdaptor) *Order {
	return &Order{
		db: adaptor.GetDB(),
	}
}

func (o *Order) CreateOrder(ctx context.Context, req *do.CreateOrder) error {
	timeNow := time.Now()
	order := &model.Order{
		ID:             req.OrderID,
		UserID:         req.UserID,
		Status:         consts.OrderStatusWaitPay,
		OrderSource:    req.OrderSource,
		OrderAmount:    req.TotalFee,
		PaymentAmount:  req.TotalPayFee,
		TradeNo:        "",
		InnerTradeNo:   req.OutTradeNo,
		OrderDesc:      req.OrderDesc,
		DiscountAmount: req.TotalDiscountFee,
		UserRemark:     req.UserRemark,
		CreateAt:       timeNow.UnixMilli(),
		CreateBy:       req.UserID,
	}

	orderItems := make([]*model.OrderItem, 0)
	for _, v := range req.Items {
		orderItems = append(orderItems, &model.OrderItem{
			OrderID:        order.ID,
			GoodsID:        v.CourseID,
			GoodsType:      consts.CourseGood,
			DiscountAmount: v.DiscountFee,
			PaymentAmount:  v.PayFee,
			GoodsSnap:      gconv.String(v.GoodsSnap),
			Quantity:       1,
			UserID:         req.UserID,
		})
	}

	return o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Create(order).Error
		if err != nil {
			return err
		}
		return tx.Create(orderItems).Error
	})
}

func (o *Order) GetOrderByID(ctx context.Context, orderID int64) (*model.Order, error) {
	qs := query.Use(o.db).Order
	return qs.WithContext(ctx).Where(qs.ID.Eq(orderID)).First()
}

func (o *Order) GetOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*model.Order, error) {
	qs := query.Use(o.db).Order
	return qs.WithContext(ctx).Where(qs.InnerTradeNo.Eq(outTradeNo)).First()
}

func (o *Order) GetOrderItems(ctx context.Context, orderID int64) ([]*model.OrderItem, error) {
	qs := query.Use(o.db).OrderItem
	return qs.WithContext(ctx).Where(qs.OrderID.Eq(orderID)).Find()
}

func (o *Order) GetOrderItemByOrderIDs(ctx context.Context, orderIds []int64) (map[int64][]*model.OrderItem, error) {
	qs := query.Use(o.db).OrderItem
	list, err := qs.WithContext(ctx).Where(qs.OrderID.In(orderIds...)).Find()
	if err != nil {
		return nil, err
	}
	retGroup := lo.GroupBy(list, func(item *model.OrderItem) int64 {
		return item.OrderID
	})
	return retGroup, nil
}

func (o *Order) GetOrderRefunds(ctx context.Context, orderID int64) ([]*model.OrderRefund, error) {
	qs := query.Use(o.db).OrderRefund
	return qs.WithContext(ctx).Where(qs.OrderID.Eq(orderID)).Find()
}

func (o *Order) CancelOrder(ctx context.Context, req *do.CancelOrder) error {
	qs := query.Use(o.db).Order
	updateMap := map[string]interface{}{
		qs.Status.ColumnName().String():       consts.OrderStatusCancel,
		qs.CancelType.ColumnName().String():   req.CancelType,
		qs.CancelBy.ColumnName().String():     req.CancelBy,
		qs.CancelAt.ColumnName().String():     req.CancelAt,
		qs.CancelReason.ColumnName().String(): req.Reason,
	}
	_, err := qs.WithContext(ctx).Where(qs.ID.Eq(req.OrderID)).Updates(updateMap)
	return err
}

func (o *Order) UpdateOrderPaySuccess(ctx context.Context, req *do.UpdateOrderPaySuccess) error {
	qs := query.Use(o.db).Order
	return o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateMap := map[string]interface{}{
			qs.Status.ColumnName().String():    consts.OrderStatusPayed,
			qs.PaymentAt.ColumnName().String(): req.PaymentAt.UnixMilli(),
			qs.TradeNo.ColumnName().String():   req.TransactionID,
		}
		res := tx.Where(qs.ID.Eq(req.OrderID), qs.Status.Eq(consts.OrderStatusWaitPay)).Updates(updateMap)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		return req.BenefitFunc(tx)
	})
}

func (o *Order) UpdateOrderOutTradeNo(ctx context.Context, orderID int64, outTradeNo string) error {
	qs := query.Use(o.db).Order
	_, err := qs.WithContext(ctx).Where(qs.ID.Eq(orderID)).Update(qs.InnerTradeNo, outTradeNo)
	return err
}

func (o *Order) GetOrderList(ctx context.Context, req *do.GetOrderList) ([]*model.Order, int64, error) {
	qs := query.Use(o.db).Order
	tx := qs.WithContext(ctx)
	if req.Status != 0 {
		tx = tx.Where(qs.Status.Eq(req.Status))
	}
	if req.OrderID != 0 {
		tx = tx.Where(qs.ID.Eq(req.OrderID))
	}
	if req.UserID != 0 {
		tx = tx.Where(qs.UserID.Eq(req.UserID))
	}
	if req.StartTime != 0 && req.EndTime != 0 {
		tx = tx.Where(qs.CreateAt.Between(req.StartTime, req.EndTime))
	}
	if req.GoodsNameKw != "" {
		tx = tx.Where(qs.OrderDesc.Like(tools.GetAllLike(req.GoodsNameKw)))
	}
	return tx.Order(qs.ID.Desc()).FindByPage(req.GetOffset(), req.Limit)
}
func (o *Order) dateSelect(dateType int32) string {
	switch dateType {
	case consts.DateTypeYear:
		return "DATE_FORMAT(FROM_UNIXTIME(orders.payment_at/1000), '%Y')"
	case consts.DateTypeQuarter:
		return "CONCAT(DATE_FORMAT(FROM_UNIXTIME(orders.payment_at/1000), '%Y'), '-Q', QUARTER(FROM_UNIXTIME(orders.payment_at/1000)))"
	case consts.DateTypeMonth:
		return "DATE_FORMAT(FROM_UNIXTIME(orders.payment_at/1000), '%Y-%m')"
	default:
		return "DATE_FORMAT(FROM_UNIXTIME(orders.payment_at/1000), '%Y-%m-%d')"
	}
}

func (o *Order) OrderStatistic(ctx context.Context, req *do.OrderStat) ([]*do.OrderStatistic, int64, error) {
	var list []*do.OrderStatistic
	tx := o.db.WithContext(ctx).
		Model(&model.Order{}).
		Select(
			o.dateSelect(req.DateType) + " AS date," +
				"COUNT(DISTINCT(orders.id)) AS order_count," +
				"SUM(orders.order_amount) AS order_amount," +
				"SUM(orders.payment_amount) AS payment_amount," +
				"SUM(CASE WHEN orders.status = 3 THEN 1 ELSE 0 END) AS refund_count," +
				"SUM(orders.refund_amount) AS refund_amount").
		Where("orders.status != -1")

	if req.GoodsID != 0 {
		existSubQuery := o.db.Model(&model.OrderItem{}).
			Select("1").
			Where("order_items.goods_id = ?", req.GoodsID)
		tx = tx.Where("EXISTS (?)", existSubQuery)
	}
	tx = tx.Where("orders.payment_at BETWEEN ? AND ?", req.StartTime, req.EndTime).
		Group(o.dateSelect(req.DateType)).
		Order(o.dateSelect(req.DateType) + " DESC")

	var count int64
	if err := tx.Count(&count).Error; err != nil {
		return nil, 0, err
	}
	if err := tx.Offset(req.GetOffset()).Limit(req.Limit).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, count, nil
}

func (o *Order) OrderRefund(ctx context.Context, req *do.OrderRefund) (int64, error) {
	refund := &model.OrderRefund{
		UserID:       req.UserID,
		ApplyUserID:  req.AdminUserID,
		OrderID:      req.OrderID,
		ItemIds:      gconv.String(req.ItemIds),
		Status:       consts.RefundStatusProcessing,
		Reason:       req.Reason,
		Amount:       req.Amount,
		ApplyAt:      time.Now().UnixMilli(),
		InnerTradeNo: req.OutTradeNo,
	}
	err := o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Create(refund).Error
		if err != nil {
			return err
		}
		return nil
	})
	err = req.RefundFun(ctx)
	if err != nil {
		qs := query.Use(o.db).OrderRefund
		updateMap := map[string]interface{}{
			qs.Status.ColumnName().String(): consts.RefundStatusException,
		}
		if _, uerr := qs.WithContext(ctx).Where(qs.ID.Eq(refund.ID)).Updates(updateMap); uerr != nil {
			return refund.ID, fmt.Errorf("refund status update failed: %w, refund fun: %w", uerr, err)
		}
		return refund.ID, err
	}
	return refund.ID, err
}

func (o *Order) GetOrderRefundByOrderID(ctx context.Context, orderID int64) (*model.OrderRefund, error) {
	qs := query.Use(o.db).OrderRefund
	return qs.WithContext(ctx).Where(qs.OrderID.Eq(orderID)).First()
}
func (o *Order) GetOrderRefundByOutTradeNo(ctx context.Context, outTradeNo string) (*model.OrderRefund, error) {
	qs := query.Use(o.db).OrderRefund
	return qs.WithContext(ctx).Where(qs.InnerTradeNo.Eq(outTradeNo)).First()
}

func (o *Order) OrderRefundResult(ctx context.Context, req *do.OrderRefundResult) error {
	qs := query.Use(o.db).OrderRefund
	updateMap := map[string]interface{}{
		qs.Status.ColumnName().String(): req.Status,
	}
	if req.Status == consts.RefundStatusDone {
		updateMap[qs.DoneAt.ColumnName().String()] = req.SuccessTime
		updateMap[qs.RefundID.ColumnName().String()] = req.RefundID
	}
	return o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&model.OrderRefund{}).
			Where(qs.ID.Eq(req.OrderRefundID)).
			Updates(updateMap).Error
		if err != nil {
			return err
		}
		return req.RefundDeliveryFun(ctx, tx)
	})
}
