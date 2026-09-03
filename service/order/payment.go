package order

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-pay/gopay/wechat/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"mall/adaptor/repo/model"
	"mall/consts"
	"mall/service/do"
	"mall/utils/logger"
	"mall/utils/tools"
	"time"
)

const (
	WechatPaySuccess = "SUCCESS"
)

func (s *Service) WechatPaymentCallback(ctx context.Context, notifyReq *wechat.V3NotifyReq) error {
	certMap := s.payment.GetPublicKeyMap()
	err := notifyReq.VerifySignByPKMap(certMap)
	if err != nil {
		logger.Error("WechatPaymentCallback VerifySignByPKMap error", zap.Error(err), zap.Any("notifyReq", notifyReq))
		return err
	}
	result := &wechat.V3DecryptPayResult{}
	err = notifyReq.DecryptCipherTextToStruct(s.conf.WechatPay.ApiKey, result)
	if err != nil {
		logger.Error("WechatPaymentCallback DecryptCipherTextToStruct error", zap.Error(err), zap.Any("notifyReq", notifyReq))
		return err
	}
	if !IsPayed(result.TradeState) {
		return errors.New("order not payment")
	}
	order, err := s.order.GetOrderByOutTradeNo(ctx, result.OutTradeNo)
	if err != nil {
		logger.Error("WechatPaymentCallback GetOrderByOutTradeNo error", zap.Error(err), zap.Any("notifyReq", notifyReq))
		return err
	}
	orderUUID := tools.UUIDHex()
	locked, err := s.rdsOrder.GetOrderLock(ctx, order.ID, orderUUID)
	if err != nil {
		logger.Error("WechatPaymentCallback GetOrderLock error", zap.Error(err), zap.Any("order_id", order.ID))
		return err
	}
	if !locked {
		logger.Error("WechatPaymentCallback other processing", zap.Error(err), zap.Any("order_id", order.ID))
		return err
	}
	defer s.rdsOrder.UnLockOrder(ctx, order.ID, orderUUID)
	stopRenew, err := s.rdsOrder.RenewOrderLockLoop(ctx, order.ID, orderUUID, consts.RenewInterval, consts.OrderLockTTL)
	if err != nil {
		logger.Error("RenewOrderLockLoop start error", zap.Error(err), zap.Int64("order_id", order.ID))
	} else {
		defer stopRenew()
	}

	paymentTime, err := time.Parse(time.RFC3339, result.SuccessTime)
	if err != nil {
		paymentTime = time.Now()
	}
	outboxFunc := s.buildOutboxFunc(ctx, order)
	err = s.order.UpdateOrderPaySuccess(ctx, &do.UpdateOrderPaySuccess{
		OrderID:       order.ID,
		PaymentAt:     paymentTime,
		TradeType:     result.TradeType,
		TransactionID: result.TransactionId,
		OutboxFunc:    outboxFunc,
	})
	if err != nil {
		logger.Error("WechatPaymentCallback UpdateOrderPaySuccess error", zap.Error(err), zap.Any("order_id", order.ID))
		return err
	}
	err = s.rdsOrder.DelTimeoutOrderCancel(ctx, order.ID)
	if err != nil {
		logger.Error("WechatPaymentCallback DelTimeoutOrderCancel error", zap.Error(err), zap.Any("order_id", order.ID))
	}
	err = s.rdsOrder.DelOrderPayResult(ctx, order.ID)
	if err != nil {
		logger.Error("WechatPaymentCallback DelOrderPayResult error", zap.Error(err), zap.Any("order_id", order.ID))
	}
	return nil
}

func IsPayed(state string) bool {
	return state == WechatPaySuccess
}
func (s *Service) QueryOrderPayResult(ctx context.Context, orderID, orderTime int64) error {
	orderUUID := tools.UUIDHex()
	locked, err := s.rdsOrder.GetOrderLock(ctx, orderID, orderUUID)
	if err != nil {
		logger.Error("QueryOrderPayResult GetOrderLock error", zap.Error(err), zap.Any("order_id", orderID))
		return err
	}
	if !locked {
		logger.Error("QueryOrderPayResult other processing", zap.Error(err), zap.Any("order_id", orderID))
		return err
	}
	defer s.rdsOrder.UnLockOrder(ctx, orderID, orderUUID)

	order, err := s.order.GetOrderByID(ctx, orderID)
	if err != nil {
		logger.Error("QueryOrderPayResult GetOrderByID error", zap.Error(err), zap.Any("order_id", orderID))
		return err
	}
	if IsWaitPay(order) {
		err = s.handlerOrderPayResult(ctx, order, orderTime)
		if err != nil {
			logger.Error("QueryOrderPayResult handlerOrderPayResult error", zap.Error(err), zap.Any("order_id", orderID))
			return err
		}
	}
	err = s.rdsOrder.DelTimeoutOrderCancel(ctx, orderID)
	if err != nil {
		logger.Error("QueryOrderPayResult DelTimeoutOrderCancel error", zap.Error(err), zap.Any("order_id", orderID))
	}
	err = s.rdsOrder.DelOrderPayResult(ctx, orderID)
	if err != nil {
		logger.Error("TimeOutOrderCancel DelOrderPayResult error", zap.Error(err), zap.Any("order_id", orderID))
	}
	return nil
}

func (s *Service) handlerOrderPayResult(ctx context.Context, order *model.Order, orderTime int64) error {
	resp, err := s.payment.QueryOrderByOutTradeNo(ctx, order.InnerTradeNo)
	if err != nil {
		logger.Error("handlerOrderPayResult QueryOrderByOutTradeNo error", zap.Error(err), zap.Any("order_id", order.ID))
		return err
	}
	if !IsPayed(resp.TradeState) {
		return errors.New("order not payment")
	}
	paymentTime, err := time.Parse(time.RFC3339, resp.SuccessTime)
	if err != nil {
		paymentTime = time.Now()
	}
	outboxFunc := s.buildOutboxFunc(ctx, order)
	err = s.order.UpdateOrderPaySuccess(ctx, &do.UpdateOrderPaySuccess{
		OrderID:       order.ID,
		PaymentAt:     paymentTime,
		TradeType:     resp.TradeType,
		TransactionID: resp.TransactionId,
		OutboxFunc:    outboxFunc,
	})
	if err != nil {
		logger.Error("TimeOutOrderCancel UpdateOrderPaySuccess error", zap.Error(err), zap.Any("order_id", order.ID))
		return err
	}
	return nil
}

func (s *Service) userBenefitPackage(ctx context.Context, tx *gorm.DB, order *model.Order, paymentTime time.Time) error {
	orderItems, err := s.order.GetOrderItems(ctx, order.ID)
	if err != nil {
		logger.Error("userBenefitPackage GetOrderItems error", zap.Error(err), zap.Any("order_id", order.ID))
		return err
	}
	buyGoods := make([]do.BuyCourseGoods, 0)
	for _, item := range orderItems {
		courseGoods := &model.CourseGood{}
		err = json.Unmarshal([]byte(item.GoodsSnap), courseGoods)
		if err != nil {
			logger.Error("userBenefitPackage Unmarshal error", zap.Error(err), zap.Any("order_id", order.ID))
			return err
		}
		learnTime := consts.GetExpireTime(courseGoods.LearnTime)
		serviceTime := consts.GetExpireTime(courseGoods.ServiceTime)
		if learnTime == 0 || serviceTime == 0 {
			return errors.New("invalid expire time")
		}
		buyGoods = append(buyGoods, do.BuyCourseGoods{
			OrderItemID:       item.ID,
			GoodsID:           item.GoodsID,
			GoodType:          item.GoodsType,
			LearnExpireTime:   learnTime,
			ServiceExpireTime: serviceTime,
		})
	}
	err = s.userCourse.CreateUserCourse(ctx, tx, &do.CreateUserCourse{
		UserId:     order.UserID,
		OrderID:    order.ID,
		BuyTime:    paymentTime.UnixMilli(),
		CourseList: buyGoods,
	})
	if err != nil {
		logger.Error("userBenefitPackage CreateUserCourse error", zap.Error(err), zap.Any("order_id", order.ID))
		return err
	}
	return nil
}

func (s *Service) buildOutboxFunc(ctx context.Context, order *model.Order) func(tx *gorm.DB) error {
	return func(tx *gorm.DB) error {
		payload, err := json.Marshal(map[string]interface{}{
			"order_id": order.ID,
			"user_id":  order.UserID,
		})
		if err != nil {
			return err
		}
		return s.outbox.CreateOutbox(ctx, tx, consts.EventOrderPayed, string(payload))
	}
}
