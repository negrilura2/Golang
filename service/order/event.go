package order

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

func (s *Service) HandleOrderPayedEvent(ctx context.Context, value []byte) error {
	//1、解析payload
	var payload struct {
		OrderID int64 `json:"order_id"`
		UserID  int64 `json:"user_id"`
	}
	if err := json.Unmarshal(value, &payload); err != nil {
		return err
	}
	//2、拿订单
	order, err := s.order.GetOrderByID(ctx, payload.OrderID)
	if err != nil {
		return err
	}
	//3、防御校验
	if order.UserID != payload.UserID {
		return errors.New("pay event user_id mismatch")
	}
	//4、发权益，tx传nil(没有多表事务） - CreateUserCourse 自己连接 + upsert幂等
	return s.userBenefitPackage(ctx, nil, order, time.UnixMilli(order.PaymentAt))
}
