package order

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-pay/gopay"
	"github.com/go-redis/redis"
	"github.com/gogf/gf/util/gconv"
	"github.com/jinzhu/copier"
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

func (s *Service) OrderCalcFee(ctx context.Context, user *common.UserInfo, req *dto.OrderCalcFeeReq) (*dto.OrderCalcFeeResp, common.Errno) {
	req.CourseIDs = lo.Uniq(req.CourseIDs)
	if len(req.CourseIDs) == 0 {
		return nil, common.ParamErr.WithMsg("course ids is empty")
	}
	courseList, err := s.course.GetCourseInfoByIds(ctx, req.CourseIDs)
	if err != nil {
		logger.Error("OrderCalcFee GetCourseInfoByIds error", zap.Error(err), zap.Any("req", req))
		return nil, common.DatabaseErr.WithErr(err)
	}
	courseMap := lo.SliceToMap(courseList, func(course *model.CourseGood) (int64, *model.CourseGood) {
		return course.ID, course
	})

	var (
		courseFees       = make([]*dto.CourseFeeDto, 0)
		totalDiscountFee int64
		totalFee         int64
		totalPayFee      int64
	)
	for _, courseID := range req.CourseIDs {
		course, ok := courseMap[courseID]
		if !ok {
			return nil, common.ParamErr.WithMsg("course goods not found")
		}
		discountFee := int64(0)
		feeDto := &dto.CourseFeeDto{
			CourseID:    course.ID,
			Price:       course.CoursePrice,
			DiscountFee: discountFee,
			PayFee:      course.CoursePrice,
			GoodsSnap:   course,
		}
		courseFees = append(courseFees, feeDto)
		totalDiscountFee = totalDiscountFee + feeDto.DiscountFee
		totalFee = totalFee + feeDto.Price
		totalPayFee = totalPayFee + feeDto.PayFee
	}

	feeUUID := tools.UUIDHex()
	orderFeeDto := &dto.OrderCalcFeeResp{
		CourseFees:       courseFees,
		FeeUUID:          feeUUID,
		TotalDiscountFee: totalDiscountFee,
		TotalFee:         totalFee,
		TotalPayFee:      totalPayFee,
	}
	if err := s.rdsOrder.SetOrderCalcFee(ctx, feeUUID, gconv.String(orderFeeDto), consts.OrderCalcFeeExpire); err != nil {
		logger.Error("OrderCalcFee SetOrderCalcFee error", zap.Error(err), zap.Any("req", req))
		return nil, common.DatabaseErr.WithErr(err)
	}
	return orderFeeDto, common.OK
}

func (s *Service) getWechatPrePay(openID, appID string, fee *dto.OrderCalcFeeResp) (gopay.BodyMap, string) {
	payAmount := fee.TotalPayFee
	if !s.conf.WechatPay.IsProd {
		payAmount = 2
	}
	outTradeNo := tools.UUIDHex()
	bodyMap := make(gopay.BodyMap)
	return bodyMap.Set("appid", appID).
		Set("description", fee.GetShortDesc()).
		Set("out_trade_no", outTradeNo).
		Set("notify_url", s.conf.WechatPay.CallbackUrl+"/payment").
		SetBodyMap("amount", func(b gopay.BodyMap) {
			b.Set("total", payAmount).
				Set("currency", "CNY")
		}).
		SetBodyMap("payer", func(b gopay.BodyMap) {
			b.Set("openid", openID)
		}), outTradeNo
}

func (s *Service) OrderPayNow(ctx context.Context, user *common.UserInfo, req *dto.OrderPayNowReq) (*dto.OrderPayNowResp, common.Errno) {
	orderCalcFeeStr, err := s.rdsOrder.GetAndDelOrderCalcFee(ctx, req.FeeUUID)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, common.OrderCalcFeeErr
		}
		logger.Error("OrderPayNow GetOrderCalcFee error", zap.Error(err), zap.Any("req", req))
		return nil, common.DatabaseErr.WithErr(err)
	}
	orderFeeDto := &dto.OrderCalcFeeResp{}
	err = json.Unmarshal([]byte(orderCalcFeeStr), orderFeeDto)
	if err != nil {
		logger.Error("OrderPayNow Unmarshal error", zap.Error(err), zap.Any("req", req))
		return nil, common.ServerErr.WithErr(err)
	}
	if orderFeeDto.TotalPayFee == 0 {
		return nil, common.OrderCalcFeeErr
	}
	orderItems := make([]*do.OrderItem, 0)
	lo.ForEach(orderFeeDto.CourseFees, func(courseFee *dto.CourseFeeDto, index int) {
		orderItems = append(orderItems, &do.OrderItem{
			CourseID:    courseFee.CourseID,
			DiscountFee: courseFee.DiscountFee,
			GoodsSnap:   courseFee.GoodsSnap,
			PayFee:      courseFee.PayFee,
		})
	})
	orderID, err := s.idNode.GetNextID()
	if err != nil {
		logger.Error("OrderPayNow GetNextID error", zap.Error(err), zap.Any("req", req))
		return nil, common.ServerErr.WithErr(err)
	}
	// 发起微信支付, 获取预支付交易号
	if len(user.AppUsers) == 0 {
		return nil, common.UserNotFoundErr
	}
	bodyMap, outTradeNo := s.getWechatPrePay(user.AppUsers[0].OpenID, s.conf.WechatPay.AppID, orderFeeDto)
	_, wxPayParams, err := s.payment.JsApiPrePayOrder(ctx, bodyMap)
	if err != nil {
		logger.Error("OrderPayNow JsApiPrePayOrder error", zap.Error(err), zap.Any("req", req))
		return nil, common.ServerErr.WithErr(err)
	}
	defer func() {
		if err != nil {
			s.payment.CloseOrder(ctx, outTradeNo)
			s.rdsOrder.DelOrderPayResult(ctx, orderID)
		}
	}()
	err = s.order.CreateOrder(ctx, &do.CreateOrder{
		OrderID:          orderID,
		UserID:           user.User.ID,
		TotalDiscountFee: orderFeeDto.TotalDiscountFee,
		OrderSource:      consts.OrderSourceUser,
		TotalFee:         orderFeeDto.TotalFee,
		TotalPayFee:      orderFeeDto.TotalPayFee,
		UserRemark:       req.Remark,
		OutTradeNo:       outTradeNo,
		OrderDesc:        orderFeeDto.GetDescription(),
		Items:            orderItems,
	})
	if err != nil {
		logger.Error("OrderPayNow CreateOrder error", zap.Error(err), zap.Any("req", req))
		return nil, common.ServerErr.WithErr(err)
	}

	err = s.rdsOrder.SetOrderPayResult(ctx, orderID)
	if err != nil {
		logger.Error("OrderPayNow SetOrderPayResult error", zap.Error(err), zap.Any("req", req))
		return nil, common.DatabaseErr.WithErr(err)
	}
	err = s.rdsOrder.SetTimeoutOrderCancel(ctx, orderID)
	if err != nil {
		logger.Error("OrderPayNow SetTimeoutOrderCancel error", zap.Error(err), zap.Any("req", req))
		return nil, common.DatabaseErr.WithErr(err)
	}
	return &dto.OrderPayNowResp{
		OrderID:   orderID,
		AppId:     wxPayParams.AppId,
		TimeStamp: wxPayParams.TimeStamp,
		NonceStr:  wxPayParams.NonceStr,
		Package:   wxPayParams.Package,
		SignType:  wxPayParams.SignType,
		PaySign:   wxPayParams.PaySign,
	}, common.OK
}

func (s *Service) OrderPayLater(ctx context.Context, user *common.UserInfo, req *dto.OrderPayLaterReq) (*dto.OrderPayNowResp, common.Errno) {
	orderUUID := tools.UUIDHex()
	locked, err := s.rdsOrder.GetOrderLock(ctx, req.OrderID, orderUUID)
	if err != nil {
		logger.Error("OrderPayLater GetOrderLock error", zap.Error(err), zap.Any("req", req))
		return nil, common.DatabaseErr.WithErr(err)
	}
	if !locked {
		return nil, common.OrderLockedErr
	}
	defer s.rdsOrder.UnLockOrder(ctx, req.OrderID, orderUUID)

	paymentIng, err := s.rdsOrder.CheckInPayment(ctx, req.OrderID)
	if err != nil && !errors.Is(err, redis.Nil) {
		logger.Error("OrderPayLater CheckInPayment error", zap.Error(err), zap.Any("req", req))
		return nil, common.ServerErr.WithErr(err)
	}
	if paymentIng {
		return nil, common.OrderPayingErr
	}
	order, err := s.order.GetOrderByID(ctx, req.OrderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.OrderNotFoundErr
		}
		logger.Error("OrderPayLater GetOrderByID error", zap.Error(err), zap.Any("req", req))
		return nil, common.DatabaseErr.WithErr(err)
	}
	orderItems, err := s.order.GetOrderItems(ctx, req.OrderID)
	if err != nil {
		logger.Error("OrderPayLater GetOrderItems error", zap.Error(err), zap.Any("req", req))
		return nil, common.DatabaseErr.WithErr(err)
	}
	courseFeeDtos := make([]*dto.CourseFeeDto, 0)
	lo.ForEach(orderItems, func(orderItem *model.OrderItem, index int) {
		courseFeeDtos = append(courseFeeDtos, &dto.CourseFeeDto{
			CourseID:    orderItem.GoodsID,
			DiscountFee: orderItem.DiscountAmount,
			GoodsSnap:   orderItem.GoodsSnap,
			PayFee:      orderItem.PaymentAmount,
		})
	})
	orderFeeDto := &dto.OrderCalcFeeResp{
		CourseFees:       courseFeeDtos,
		TotalDiscountFee: order.DiscountAmount,
		TotalFee:         order.OrderAmount,
		TotalPayFee:      order.PaymentAmount,
	}
	// 发起微信支付, 获取预支付交易号
	if len(user.AppUsers) == 0 {
		return nil, common.UserNotFoundErr
	}
	bodyMap, outTradeNo := s.getWechatPrePay(user.AppUsers[0].OpenID, s.conf.WechatPay.AppID, orderFeeDto)
	_, wxPayParams, err := s.payment.JsApiPrePayOrder(ctx, bodyMap)
	if err != nil {
		logger.Error("OrderPayLater JsApiPrePayOrder error", zap.Error(err), zap.Any("req", req))
		return nil, common.ServerErr.WithErr(err)
	}
	defer func() {
		if err != nil {
			s.payment.CloseOrder(ctx, outTradeNo)
		}
	}()
	err = s.order.UpdateOrderOutTradeNo(ctx, order.ID, outTradeNo)
	if err != nil {
		logger.Error("OrderPayLater UpdateOrderOutTradeNo error", zap.Error(err), zap.Any("req", req))
		return nil, common.DatabaseErr.WithErr(err)
	}
	err = s.rdsOrder.SetOrderPayResult(ctx, order.ID)
	if err != nil {
		logger.Error("OrderPayLater SetOrderPayResult error", zap.Error(err), zap.Any("req", req))
		return nil, common.DatabaseErr.WithErr(err)
	}
	return &dto.OrderPayNowResp{
		OrderID:   order.ID,
		AppId:     wxPayParams.AppId,
		TimeStamp: wxPayParams.TimeStamp,
		NonceStr:  wxPayParams.NonceStr,
		Package:   wxPayParams.Package,
		SignType:  wxPayParams.SignType,
		PaySign:   wxPayParams.PaySign,
	}, common.OK
}

//	func (s *Service) CancelOrder(ctx context.Context, user *common.UserInfo, req *dto.CancelOrderReq) common.Errno {
//		orderUUID := tools.UUIDHex()
//		locked, err := s.rdsOrder.GetOrderLock(ctx, req.OrderID, orderUUID)
//		if err != nil {
//			logger.Error("CancelOrder GetOrderLock error", zap.Error(err), zap.Any("req", req))
//			return common.DatabaseErr.WithErr(err)
//		}
//		if !locked {
//			return common.OrderLockedErr
//		}
//		defer s.rdsOrder.UnLockOrder(ctx, req.OrderID, orderUUID)
//
//		order, err := s.order.GetOrderByID(ctx, req.OrderID)
//		if err != nil {
//			if errors.Is(err, gorm.ErrRecordNotFound) {
//				return common.OrderNotFoundErr
//			}
//			logger.Error("CancelOrder GetOrderByID error", zap.Error(err), zap.Any("req", req))
//			return common.DatabaseErr.WithErr(err)
//		}
//		if order.Status != consts.OrderStatusWaitPay {
//			return common.OrderCantCancelErr
//		}
//		err = s.order.CancelOrder(ctx, &do.CancelOrder{
//			OrderID:    req.OrderID,
//			CancelType: consts.CustomerUser,
//			CancelBy:   user.User.ID,
//			CancelAt:   time.Now().UnixMilli(),
//			Reason:     req.Reason,
//		})
//		if err != nil {
//			logger.Error("CancelOrder error", zap.Error(err), zap.Any("req", req))
//			return common.ServerErr.WithErr(err)
//		}
//		s.payment.CloseOrder(ctx, order.InnerTradeNo)
//		s.rdsOrder.DelOrderPayResult(ctx, order.ID)
//		s.rdsOrder.DelTimeoutOrderCancel(ctx, order.ID)
//
//		return common.OK
//	}
func (s *Service) CancelOrder(ctx context.Context, user *common.UserInfo, req *dto.CancelOrderReq) common.Errno {
	orderUUID := tools.UUIDHex()
	locked, err := s.rdsOrder.GetOrderLock(ctx, req.OrderID, orderUUID)
	if err != nil {
		logger.Error("CancelOrderError GetOrderLockError", zap.Error(err), zap.Int64("order", req.OrderID))
		return common.DatabaseErr.WithErr(err)
	}
	if !locked {
		logger.Error("CancelOrderError GetOrderLockError", zap.Int64("order", req.OrderID))
		return common.OrderLockedErr
	}
	defer s.rdsOrder.UnLockOrder(ctx, req.OrderID, orderUUID)

	order, err := s.order.GetOrderByID(ctx, req.OrderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Error("CancelOrderError GetOrderByIDError", zap.Error(err), zap.Int64("order", req.OrderID))
			return common.OrderNotFoundErr
		}
		logger.Error("CancelOrderError GetOrderByIDError", zap.Error(err), zap.Int64("order", req.OrderID))
		return common.DatabaseErr.WithErr(err)
	}
	if order.Status != consts.OrderStatusWaitPay {
		return common.OrderCantCancelErr
	}
	err = s.order.CancelOrder(ctx, &do.CancelOrder{
		OrderID:    req.OrderID,
		CancelType: consts.CustomerUser,
		CancelBy:   user.User.ID,
		CancelAt:   time.Now().UnixMilli(),
		Reason:     req.Reason,
	})
	if err != nil {
		logger.Error("CancelOrder CancelOrder error", zap.Error(err), zap.Int64("order_id", req.OrderID))
		return common.ServerErr.WithErr(err)
	}

	err = s.payment.CloseOrder(ctx, order.InnerTradeNo)
	if err != nil {
		logger.Error("payment CloseOrderError", zap.Error(err), zap.Int64("order_id", req.OrderID))
	}
	err = s.rdsOrder.DelOrderPayResult(ctx, order.ID)
	if err != nil {
		logger.Error("DelOrderPayResult", zap.Error(err), zap.Int64("order_id", req.OrderID))
	}
	s.rdsOrder.DelTimeoutOrderCancel(ctx, order.ID)
	if err != nil {
		logger.Error("DelTimeoutOrderCancelError", zap.Error(err), zap.Int64("order_id", req.OrderID))
	}
	return common.OK
}
func (s *Service) GetUserOrderList(ctx context.Context, user *common.UserInfo, req *dto.GetOrderListReq) (*dto.GetUserOrderListResp, common.Errno) {
	list, count, err := s.order.GetOrderList(ctx, &do.GetOrderList{
		Pager:       req.Pager,
		Status:      req.Status,
		OrderID:     req.OrderID,
		UserID:      user.User.ID,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		GoodsNameKw: req.GoodsNameKw,
	})
	if err != nil {
		logger.Error("GetUserOrderList error", zap.Error(err), zap.Any("req", req))
		return nil, common.DatabaseErr.WithErr(err)
	}
	orderIds := lo.Map(list, func(item *model.Order, index int) int64 {
		return item.ID
	})
	orderItemsMap, err := s.order.GetOrderItemByOrderIDs(ctx, orderIds)
	if err != nil {
		logger.Error("GetUserOrderList GetOrderItemByOrderIDs error", zap.Error(err), zap.Any("req", req))
		return nil, common.DatabaseErr.WithErr(err)
	}
	fileKeys := make([]string, 0)
	goodsSnapMap := make(map[int64]*do.GoodsSnap)
	for _, items := range orderItemsMap {
		lo.ForEach(items, func(item *model.OrderItem, index int) {
			goodsSnap := &do.GoodsSnap{}
			_ = json.Unmarshal([]byte(item.GoodsSnap), goodsSnap)
			fileKeys = append(fileKeys, goodsSnap.CoverKey, goodsSnap.DetailCoverKey)
			goodsSnapMap[item.ID] = goodsSnap
		})
	}
	fileUrlMap, err := s.storage.GetPreviewUrl(ctx, &do.GetPreviewUrl{
		Keys:        fileKeys,
		ExpireHours: 6,
	})
	if err != nil {
		logger.Error("GetUserOrderList GetPreviewUrl error", zap.Error(err), zap.Any("req", req))
		return nil, common.ServerErr.WithErr(err)
	}
	for k, v := range goodsSnapMap {
		goodsSnapMap[k].CoverUrl = fileUrlMap[v.CoverKey]
		goodsSnapMap[k].DetailCoverUrl = fileUrlMap[v.DetailCoverKey]
	}
	orderList := s.convertModelOrderToOrderDto(ctx, list)
	orderMap := lo.SliceToMap(orderList, func(item *dto.OrderDto) (int64, *dto.OrderDto) {
		return item.ID, item
	})
	retList := make([]*dto.OrderInfoResp, 0)
	lo.ForEach(list, func(order *model.Order, index int) {
		modelItems, ok := orderItemsMap[order.ID]
		orderItems := make([]*dto.OrderItemDto, 0)
		if ok {
			copier.Copy(&orderItems, modelItems)
		}
		for i, item := range orderItems {
			orderItems[i].GoodsSnap = goodsSnapMap[item.ID]
		}
		retList = append(retList, &dto.OrderInfoResp{
			OrderDto: orderMap[order.ID],
			Items:    orderItems,
		})
	})
	return &dto.GetUserOrderListResp{
		List:  retList,
		Total: count,
		Pager: req.Pager,
	}, common.OK
}
