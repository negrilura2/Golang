package order

import (
	"context"
	"go.uber.org/zap"
	"mall/adaptor/repo/model"
	"mall/consts"
	"mall/service/do"
	"mall/utils/logger"
	"mall/utils/tools"
	"time"
)

func IsWaitPay(order *model.Order) bool {
	if order == nil {
		return false
	}
	return order.Status == consts.OrderStatusWaitPay
}

func (s *Service) TimeOutOrderCancel(ctx context.Context, orderID int64) error {
	orderUUID := tools.UUIDHex()
	locked, err := s.rdsOrder.GetOrderLock(ctx, orderID, orderUUID)
	if err != nil {
		logger.Error("TimeOutOrderCancel GetOrderLock error", zap.Error(err), zap.Any("order_id", orderID))
		return err
	}
	if !locked {
		logger.Error("TimeOutOrderCancel other processing", zap.Error(err), zap.Any("order_id", orderID))
		return err
	}
	defer s.rdsOrder.UnLockOrder(ctx, orderID, orderUUID)
	stopRenew, err := s.rdsOrder.RenewOrderLockLoop(ctx, orderID, orderUUID, consts.RenewInterval, consts.OrderLockTTL)
	if err != nil {
		logger.Error("RenewOrderLockLoop start error", zap.Error(err), zap.Int64("order_id", orderID))
	} else {
		defer stopRenew()
	}
	order, err := s.order.GetOrderByID(ctx, orderID)
	if err != nil {
		logger.Error("TimeOutOrderCancel GetOrderByID error", zap.Error(err), zap.Any("order_id", orderID))
		return err
	}
	if IsWaitPay(order) {
		err = s.order.CancelOrder(ctx, &do.CancelOrder{
			OrderID:    orderID,
			CancelType: consts.CancelTypeTimeout,
			CancelBy:   consts.SystemCancelBy,
			CancelAt:   time.Now().UnixMilli(),
		})
		if err != nil {
			logger.Error("TimeOutOrderCancel CancelOrder error", zap.Error(err), zap.Any("order_id", orderID))
			return err
		}
	}
	err = s.rdsOrder.DelTimeoutOrderCancel(ctx, orderID)
	if err != nil {
		logger.Error("TimeOutOrderCancel DelTimeoutOrderCancel error", zap.Error(err), zap.Any("order_id", orderID))
	}
	err = s.rdsOrder.DelOrderPayResult(ctx, orderID)
	if err != nil {
		logger.Error("TimeOutOrderCancel DelOrderPayResult error", zap.Error(err), zap.Any("order_id", orderID))
	}
	return nil
}
