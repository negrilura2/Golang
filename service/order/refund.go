package order

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/wechat/v3"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"mall/adaptor/repo/model"
	"mall/common"
	"mall/consts"
	"mall/service/do"
	"mall/service/dto"
	"mall/utils/logger"
	"mall/utils/tools"
	"time"
)

func (s *Service) WechatRefundCallback(ctx context.Context, notifyReq *wechat.V3NotifyReq) error {
	certMap := s.payment.GetPublicKeyMap()
	err := notifyReq.VerifySignByPKMap(certMap)
	if err != nil {
		logger.Error("WechatRefundCallback VerifySignByPKMap error", zap.Error(err), zap.Any("notifyReq", notifyReq))
		return err
	}
	result := &wechat.V3DecryptRefundResult{}
	err = notifyReq.DecryptCipherTextToStruct(s.conf.WechatPay.ApiKey, result)
	if err != nil {
		logger.Error("WechatRefundCallback DecryptCipherTextToStruct error", zap.Error(err), zap.Any("notifyReq", notifyReq))
		return err
	}
	orderRefund, err := s.order.GetOrderRefundByOutTradeNo(ctx, result.OutTradeNo)
	if err != nil {
		logger.Error("WechatRefundCallback GetOrderRefundByOutTradeNo error", zap.Error(err), zap.Any("notifyReq", notifyReq))
		return err
	}
	orderUUID := tools.UUIDHex()
	locked, err := s.rdsOrder.GetOrderLock(ctx, orderRefund.OrderID, orderUUID)
	if err != nil {
		logger.Error("WechatRefundCallback GetOrderLock error", zap.Error(err), zap.Any("order_id", orderRefund.OrderID))
		return err
	}
	if !locked {
		logger.Error("WechatRefundCallback other processing", zap.Error(err), zap.Any("order_id", orderRefund.OrderID))
		return err
	}
	defer s.rdsOrder.UnLockOrder(ctx, orderRefund.OrderID, orderUUID)

	if orderRefund.Status != consts.RefundStatusProcessing {
		return s.rdsOrder.DelOrderRefundResult(ctx, orderRefund.OrderID)
	}

	refundStatus := consts.GetRefundStatus(result.RefundStatus)
	if refundStatus == consts.RefundStatusProcessing {
		return nil
	}
	successTime, err := time.Parse(time.RFC3339, result.SuccessTime)
	if err != nil {
		successTime = time.Now()
	}
	itemIds := make([]int64, 0)
	err = json.Unmarshal([]byte(orderRefund.ItemIds), &itemIds)
	if err != nil {
		logger.Error("WechatRefundCallback Unmarshal error", zap.Error(err), zap.Any("order_id", orderRefund.OrderID))
		return err
	}
	handleFun := func(ctx context.Context) error {
		return s.userCourse.DeleteUserCourse(ctx, &do.DeleteUserCourse{
			UserID:       orderRefund.UserID,
			OrderID:      orderRefund.OrderID,
			OrderItemIds: itemIds,
		})
	}
	err = s.order.OrderRefundResult(ctx, &do.OrderRefundResult{
		OrderRefundID:     orderRefund.ID,
		RefundID:          result.RefundId,
		Status:            refundStatus,
		SuccessTime:       successTime.UnixMilli(),
		RefundDeliveryFun: handleFun,
	})
	if err != nil {
		logger.Error("WechatRefundCallback OrderRefundResult error", zap.Error(err), zap.Any("order_id", orderRefund.OrderID))
		return common.DatabaseErr.WithErr(err)
	}
	return s.rdsOrder.DelOrderRefundResult(ctx, orderRefund.OrderID)
}

func (s *Service) enableRefund(order *model.Order) bool {
	enableList := []int32{
		consts.OrderStatusWaitPay,
		consts.OrderStatusShipped,
		consts.OrderStatusReceived,
		consts.OrderStatusCompleted,
	}
	return lo.Contains(enableList, order.Status)
}

func (s *Service) packageWechatRefund(ctx context.Context, order *model.Order, req *dto.OrderRefundReq) (gopay.BodyMap, string, common.Errno) {
	bodyMap := make(gopay.BodyMap)
	amount := req.Amount
	if amount <= 0 || amount > order.PaymentAmount {
		return nil, "", common.OrderRefundAmountErr
	}
	if !s.conf.WechatPay.IsProd {
		amount = 1
		order.PaymentAmount = 2
	}
	if len(req.Reason) == 0 {
		req.Reason = "用户要求退款"
	}
	outTradeNo := tools.UUIDHex()
	bodyMap.Set("out_refund_no", outTradeNo).
		Set("out_trade_no", order.InnerTradeNo).
		Set("notify_url", s.conf.WechatPay.CallbackUrl+"/refund").
		Set("reason", req.Reason).SetBodyMap("amount", func(b gopay.BodyMap) {
		b.Set("total", order.PaymentAmount).
			Set("refund", amount).
			Set("currency", "CNY")
	})
	return bodyMap, outTradeNo, common.OK
}
func (s *Service) OrderRefund(ctx context.Context, user *common.AdminUser, req *dto.OrderRefundReq) common.Errno {
	orderUUID := tools.UUIDHex()
	locked, err := s.rdsOrder.GetOrderLock(ctx, req.OrderID, orderUUID)
	if err != nil {
		logger.Error("OrderRefund GetOrderLock error", zap.Error(err), zap.Any("req", req))
		return common.ServerErr.WithErr(err)
	}
	if !locked {
		logger.Error("OrderRefund other processing", zap.Error(err), zap.Any("req", req))
		return common.OrderLockedErr
	}
	defer s.rdsOrder.UnLockOrder(ctx, req.OrderID, orderUUID)

	order, err := s.order.GetOrderByID(ctx, req.OrderID)
	if err != nil {
		logger.Error("OrderRefund GetOrderByID error", zap.Error(err), zap.Any("req", req))
		return common.DatabaseErr.WithErr(err)
	}
	if !s.enableRefund(order) {
		return common.OrderCantRefundErr
	}

	orderItems, err := s.order.GetOrderItems(ctx, req.OrderID)
	if err != nil {
		logger.Error("OrderRefund GetOrderItems error", zap.Error(err), zap.Any("req", req))
		return common.DatabaseErr.WithErr(err)
	}
	orderItemIds := lo.Map(orderItems, func(item *model.OrderItem, index int) int64 {
		return item.ID
	})

	bodyMap, outTradeNo, errno := s.packageWechatRefund(ctx, order, req)
	if errno.NotOk() {
		logger.Error("OrderRefund packageWechatRefund error", zap.Error(err), zap.Any("req", req))
		return errno
	}
	handleFun := func(ctx context.Context) error {
		_, err := s.payment.ApplyOrderRefund(ctx, bodyMap)
		if err != nil {
			logger.Error("OrderRefund ApplyOrderRefund error", zap.Error(err), zap.Any("req", req))
			return err
		}
		return nil
	}
	// 添加到轮询查退款是否成功的队列里面
	err = s.rdsOrder.SetOrderRefundResult(ctx, req.OrderID)
	if err != nil {
		logger.Error("OrderRefund SetOrderRefundResult error", zap.Error(err), zap.Any("req", req))
		return common.RedisErr.WithErr(err)
	}
	_, err = s.order.OrderRefund(ctx, &do.OrderRefund{
		UserID:      order.UserID,
		OrderID:     req.OrderID,
		ItemIds:     orderItemIds,
		Reason:      req.Reason,
		Amount:      req.Amount,
		AdminUserID: user.UserID,
		OutTradeNo:  outTradeNo,
		RefundFun:   handleFun,
	})
	if err != nil {
		// 如果失败了，需要删除刚刚加入的轮询队列
		s.rdsOrder.DelOrderRefundResult(ctx, req.OrderID)
		logger.Error("OrderRefund OrderRefund error", zap.Error(err), zap.Any("req", req))
		return common.DatabaseErr.WithErr(err)
	}
	return common.OK
}

func (s *Service) QueryOrderRefundResult(ctx context.Context, orderID, orderTime int64) error {
	orderUUID := tools.UUIDHex()
	locked, err := s.rdsOrder.GetOrderLock(ctx, orderID, orderUUID)
	if err != nil {
		logger.Error("QueryOrderRefundResult GetOrderLock error", zap.Error(err), zap.Any("order_id", orderID))
		return err
	}
	if !locked {
		logger.Error("QueryOrderRefundResult other processing", zap.Error(err), zap.Any("order_id", orderID))
		return err
	}
	defer s.rdsOrder.UnLockOrder(ctx, orderID, orderUUID)

	orderRefund, err := s.order.GetOrderRefundByOrderID(ctx, orderID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("QueryOrderRefundResult GetOrderRefundByOrderID error", zap.Error(err), zap.Any("order_id", orderID))
		return err
	}
	if orderRefund == nil {
		err = s.rdsOrder.DelOrderRefundResult(ctx, orderID)
		if err != nil {
			logger.Error("QueryOrderRefundResult DelOrderRefundResult error", zap.Error(err), zap.Any("order_id", orderID))
			return err
		}
		return nil
	}
	if orderRefund.Status != consts.RefundStatusProcessing {
		return s.rdsOrder.DelOrderRefundResult(ctx, orderID)
	}
	wxResp, err := s.payment.QueryOrderRefund(ctx, orderRefund.InnerTradeNo)
	if err != nil {
		logger.Error("QueryOrderRefundResult QueryOrderRefund error", zap.Error(err), zap.Any("order_id", orderID))
		return err
	}
	refundStatus := consts.GetRefundStatus(wxResp.Status)
	if refundStatus == consts.RefundStatusProcessing {
		return nil
	}
	successTime, err := time.Parse(time.RFC3339, wxResp.SuccessTime)
	if err != nil {
		successTime = time.Now()
	}
	itemIds := make([]int64, 0)
	err = json.Unmarshal([]byte(orderRefund.ItemIds), &itemIds)
	if err != nil {
		logger.Error("QueryOrderRefundResult Unmarshal error", zap.Error(err), zap.Any("order_id", orderID))
		return err
	}
	handleFun := func(ctx context.Context) error {
		return s.userCourse.DeleteUserCourse(ctx, &do.DeleteUserCourse{
			UserID:       orderRefund.UserID,
			OrderID:      orderRefund.OrderID,
			OrderItemIds: itemIds,
		})
	}
	err = s.order.OrderRefundResult(ctx, &do.OrderRefundResult{
		OrderRefundID:     orderRefund.ID,
		RefundID:          wxResp.RefundId,
		Status:            refundStatus,
		SuccessTime:       successTime.UnixMilli(),
		RefundDeliveryFun: handleFun,
	})
	if err != nil {
		logger.Error("QueryOrderRefundResult OrderRefundResult error", zap.Error(err), zap.Any("order_id", orderID))
		return common.DatabaseErr.WithErr(err)
	}
	return s.rdsOrder.DelOrderRefundResult(ctx, orderID)
}
