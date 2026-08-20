package order

import (
	"context"
	"encoding/json"
	"github.com/jinzhu/copier"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"mall/adaptor"
	"mall/adaptor/redis"
	"mall/adaptor/repo/admin"
	"mall/adaptor/repo/goods"
	"mall/adaptor/repo/model"
	"mall/adaptor/repo/order"
	"mall/adaptor/repo/user"
	"mall/adaptor/rpc"
	"mall/common"
	"mall/config"
	"mall/consts"
	"mall/service/do"
	"mall/service/dto"
	"mall/utils/logger"
	"mall/utils/pool"
)

type Service struct {
	conf       *config.Config
	course     goods.ICourse
	rdsOrder   redis.IOrder
	order      order.IOrder
	userCourse user.IUserCourse
	payment    rpc.IPay
	idNode     redis.IGenID
	adminUser  admin.IAdminUser
	user       user.IUser
	storage    rpc.IStorage
}

func NewService(adaptor adaptor.IAdaptor) *Service {
	return &Service{
		conf:     adaptor.GetConfig(),
		course:   goods.NewCourse(adaptor),
		rdsOrder: redis.NewOrder(adaptor),
		order:    order.NewOrder(adaptor),
		// 本地学习环境无微信支付商户密钥，改用 MockPay 假支付实现，
		// 使支付相关链路(pay_now/pay_later/cancel/定时查单)可完整走通。
		// 接入真实商户号并建好 payment_private_key 表后，换回：
		// payment:    rpc.NewWechatPay(adaptor),
		payment:    rpc.NewMockPay(),
		idNode:     redis.NewGenIdNode(adaptor),
		userCourse: user.NewUserCourse(adaptor),
		adminUser:  admin.NewAdminUser(adaptor),
		user:       user.NewUser(adaptor),
		storage:    rpc.NewStorage(adaptor),
	}
}

func (s *Service) convertModelOrderToOrderDto(ctx context.Context, list []*model.Order) []*dto.OrderDto {
	var (
		adminUserIDs []int64
		adminUserMap map[int64]string
		userIDs      []int64
		userMap      map[int64]string
	)
	lo.ForEach(list, func(item *model.Order, index int) {
		userIDs = append(userIDs, item.CreateBy)
		if item.CancelBy != nil {
			switch *item.CancelType {
			case consts.AdminUser:
				adminUserIDs = append(adminUserIDs, *item.CancelBy)
			case consts.CustomerUser:
				userIDs = append(userIDs, *item.CancelBy)
			}
		}
	})

	tempPool := pool.NewPoolWithSize(2)
	defer tempPool.Release()
	tempPool.RunGo(func() {
		tempMap, err := s.adminUser.GetUserNameMap(ctx, adminUserIDs)
		if err != nil {
			logger.Error("convertModelOrderToOrderDto GetUserNameMap admin error", zap.Error(err), zap.Any("adminUserIDs", adminUserIDs))
			return
		}
		adminUserMap = tempMap
	})
	tempPool.RunGo(func() {
		tempMap, err := s.user.GetUserNameMap(ctx, userIDs)
		if err != nil {
			logger.Error("convertModelOrderToOrderDto GetUserNameMap customer error", zap.Error(err), zap.Any("userIDs", userIDs))
			return
		}
		userMap = tempMap
	})
	tempPool.Wait()

	retList := make([]*dto.OrderDto, 0)
	copier.Copy(&retList, list)
	lo.ForEach(retList, func(item *dto.OrderDto, index int) {
		item.CreateName = userMap[item.CreateBy]
		if item.CancelType == consts.AdminUser {
			item.CancelName = adminUserMap[item.CancelBy]
		} else {
			item.CancelName = userMap[item.CancelBy]
		}
	})
	return retList
}

func (s *Service) GetOrderInfo(ctx context.Context, user *common.UserInfo, req *dto.GetOrderInfoReq) (*dto.OrderInfoResp, common.Errno) {
	tempOrder, err := s.order.GetOrderByID(ctx, req.OrderID)
	if err != nil {
		logger.Error("GetOrderInfo GetOrderByID error", zap.Error(err), zap.Any("req", req))
		return nil, common.DatabaseErr.WithErr(err)
	}
	if user != nil && tempOrder.UserID != user.User.ID {
		return nil, common.OK
	}
	var (
		items   []*model.OrderItem
		refunds []*model.OrderRefund
	)
	tempPool := pool.NewPoolWithSize(2)
	defer tempPool.Release()
	tempPool.RunGo(func() {
		temp, err := s.order.GetOrderItems(ctx, req.OrderID)
		if err != nil {
			logger.Error("GetOrderInfo GetOrderItems error", zap.Error(err), zap.Any("req", req))
			return
		}
		items = temp
	})
	tempPool.RunGo(func() {
		temp, err := s.order.GetOrderRefunds(ctx, req.OrderID)
		if err != nil {
			logger.Error("GetOrderInfo GetOrderRefunds error", zap.Error(err), zap.Any("req", req))
			return
		}
		refunds = temp
	})
	tempPool.Wait()

	tempList := s.convertModelOrderToOrderDto(ctx, []*model.Order{tempOrder})
	if len(tempList) == 0 {
		logger.Error("GetOrderInfo convertModelOrderToOrderDto error", zap.Any("req", req))
		return nil, common.ServerErr.WithMsg("convertModelOrderToOrderDto error")
	}
	var (
		orderDto            = tempList[0]
		orderItems          = make([]*dto.OrderItemDto, 0)
		orderRefunds        = make([]*dto.RefundDto, 0)
		refundItemsMap      = make(map[int64][]int64)
		itemRefundStatusMap = make(map[int64]int32)
	)
	refundMaps := lo.SliceToMap(refunds, func(item *model.OrderRefund) (int64, *model.OrderRefund) {
		itemIds := make([]int64, 0)
		json.Unmarshal([]byte(item.ItemIds), &itemIds)
		refundItemsMap[item.ID] = itemIds
		return item.ID, item
	})
	itemMap := lo.SliceToMap(items, func(item *model.OrderItem) (int64, *model.OrderItem) {
		return item.ID, item
	})

	copier.Copy(&orderItems, items)
	copier.Copy(&orderRefunds, refunds)
	for i, item := range orderRefunds {
		orderRefunds[i].ItemIds = refundItemsMap[item.ID]
		orderRefunds[i].Status = refundMaps[item.ID].Status
	}
	for _, refund := range refunds {
		itemIds := make([]int64, 0)
		_ = json.Unmarshal([]byte(refund.ItemIds), &itemIds)
		for _, itemID := range itemIds {
			itemRefundStatusMap[itemID] = refund.Status
		}
	}
	for _, item := range orderItems {
		orderItem, ok := itemMap[item.ID]
		if !ok {
			logger.Error("GetOrderInfo GetOrderItems error", zap.Any("req", req))
			return nil, common.ServerErr.WithMsg("GetOrderItems error")
		}
		goodsSnap := &model.CourseGood{}
		err = json.Unmarshal([]byte(orderItem.GoodsSnap), goodsSnap)
		if err != nil {
			logger.Error("GetOrderInfo json.Unmarshal error", zap.Error(err), zap.Any("orderItem", orderItem))
			return nil, common.ServerErr.WithMsg("json.Unmarshal error")
		}
		// 修复：订单无退款记录时 refundMaps[item.ID] 为 nil，直接取 .Status 会 panic。
		// 改为安全取值，无退款时 RefundStatus 保持 0（无退款）。
		item.RefundStatus = itemRefundStatusMap[item.ID]
		item.GoodsSnap = goodsSnap
	}
	resp := &dto.OrderInfoResp{
		OrderDto: orderDto,
		Items:    orderItems,
		Refunds:  orderRefunds,
	}
	return resp, common.OK
}

func (s *Service) OrderStatistic(ctx context.Context, req *dto.OrderStatReq) (*dto.OrderStatResp, common.Errno) {
	list, count, err := s.order.OrderStatistic(ctx, &do.OrderStat{
		DateType:  req.DateType,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		GoodsID:   req.GoodsID,
	})
	if err != nil {
		logger.Error("OrderStatistic OrderStatistic error", zap.Error(err), zap.Any("req", req))
		return nil, common.DatabaseErr.WithErr(err)
	}
	tempList := make([]*dto.OrderStatDto, 0)
	lo.ForEach(list, func(item *do.OrderStatistic, index int) {
		tempList = append(tempList, &dto.OrderStatDto{
			Date:         item.Date,
			OrderAmount:  item.OrderAmount,
			OrderCount:   item.OrderCount,
			PaymentCount: item.PaymentCount,
			RefundAmount: item.RefundAmount,
			RefundCount:  item.RefundCount,
		})
	})
	return &dto.OrderStatResp{
		Pager: req.Pager,
		Total: count,
		List:  tempList,
	}, common.OK
}
