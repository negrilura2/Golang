package order

import (
	"context"
	"mall/adaptor/rpc"
	"mall/common"
	"mall/service/dto"
)

func (s *Service) MockPay(ctx context.Context, req *dto.MockPayReq) common.Errno {
	mp, ok := s.payment.(*rpc.MockPay)
	if !ok {
		return common.Errno{
			Code:   40001,
			Msg:    "当前不是mock支付环境",
			ErrMsg: "环境错误",
		}
	}
	order, err := s.order.GetOrderByID(ctx, req.OrderID)
	if err != nil {
		return common.OrderNotFoundErr.WithErr(err)
	}
	mp.MarkPaid(ctx, order.InnerTradeNo)
	return common.OK
}
