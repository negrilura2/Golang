package order

import (
	"context"
	"go.uber.org/zap"
	"mall/common"
	"mall/service/do"
	"mall/service/dto"
	"mall/utils/logger"
)

func (s *Service) GetOrderList(ctx context.Context, req *dto.GetOrderListReq) (*dto.GetOrderListResp, common.Errno) {
	list, count, err := s.order.GetOrderList(ctx, &do.GetOrderList{
		Pager:       req.Pager,
		Status:      req.Status,
		OrderID:     req.OrderID,
		UserID:      req.UserID,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		GoodsNameKw: req.GoodsNameKw,
	})
	if err != nil {
		logger.Error("GetOrderList  GetOrderList error", zap.Error(err), zap.Any("req", req))
		return nil, common.DatabaseErr.WithErr(err)
	}
	return &dto.GetOrderListResp{
		List:  s.convertModelOrderToOrderDto(ctx, list),
		Total: count,
		Pager: req.Pager,
	}, common.OK
}
