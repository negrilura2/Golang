package dto

import (
	"github.com/gogf/gf/util/gconv"
	"mall/adaptor/repo/model"
	"mall/common"
	"strings"
)

type OrderCalcFeeReq struct {
	CourseIDs []int64 `json:"course_ids"`
	Platform  string  `json:"platform"`
}

type CourseFeeDto struct {
	CourseID    int64       `json:"course_id"`
	Price       int64       `json:"price"`        // 课程价格
	DiscountFee int64       `json:"discount_fee"` // 优惠金额
	PayFee      int64       `json:"pay_fee"`      // 实际支付金额
	GoodsSnap   interface{} `json:"goods_snap"`   // 商品快照
}

type OrderCalcFeeResp struct {
	FeeUUID          string          `json:"fee_uuid"`           // 本次计算费用的唯一标识
	TotalFee         int64           `json:"total_fee"`          // 订单总价
	TotalDiscountFee int64           `json:"total_discount_fee"` // 订单总优惠金额
	TotalPayFee      int64           `json:"total_pay_fee"`      // 订单应支付金额
	ExpireTime       int64           `json:"expire_time"`        // 过期时间
	CourseFees       []*CourseFeeDto `json:"course_fees"`        // 每个课程的价格明细
}

func (o *OrderCalcFeeResp) GetShortDesc() string {
	desc := o.GetDescription()
	if len(desc) > 127 {
		desc = desc[:127]
	}
	return desc
}

func (o *OrderCalcFeeResp) GetDescription() string {
	var descList []string
	for _, v := range o.CourseFees {
		goodsSnap := &model.CourseGood{}
		gconv.Struct(v.GoodsSnap, goodsSnap)
		descList = append(descList, goodsSnap.Name)
	}

	return strings.Join(descList, ",")
}

type OrderPayNowReq struct {
	FeeUUID string `json:"fee_uuid"`
	Remark  string `json:"remark"`
}

type OrderPayLaterReq struct {
	OrderID int64 `json:"order_id"`
}

type OrderPayNowResp struct {
	OrderID   int64  `json:"order_id"`
	AppId     string `json:"appId"`
	TimeStamp string `json:"timeStamp"`
	NonceStr  string `json:"nonceStr"`
	Package   string `json:"package"`
	SignType  string `json:"signType"`
	PaySign   string `json:"paySign"`
}

type CancelOrderReq struct {
	OrderID int64  `json:"order_id"`
	Reason  string `json:"reason"`
}

type GetOrderInfoReq struct {
	OrderID int64 `form:"order_id"`
}

type GetOrderListReq struct {
	common.Pager
	UserID      int64  `form:"user_id"`
	OrderID     int64  `form:"order_id"`
	Status      int32  `form:"status"`
	GoodsNameKw string `form:"goods_name_kw"`
	StartTime   int64  `form:"start_time"`
	EndTime     int64  `form:"end_time"`
}

type OrderDto struct {
	ID                  int64  `json:"id"`
	UserID              int64  `json:"user_id"`               // 用户ID
	Status              int32  `json:"status"`                // -1：已取消，1: 待支付 2：已支付（待发货） 3：已退款  4：已发货 5：已签收 6：已收货
	OrderSource         int32  `json:"order_source"`          // 1：用户下单  2：管理后台  3：系统赠送
	OrderAmount         int64  `json:"order_amount"`          // 订单金额=支付金额，单位分
	DiscountAmount      int64  `json:"discount_amount"`       // 优惠金额，单位是分
	PaymentAmount       int64  `json:"payment_amount"`        // 支付金额-实际支付金额，单位分
	TradeNo             string `json:"trade_no"`              // 支付平台订单号
	InnerTradeNo        string `json:"inner_trade_no"`        // 内部支付订单号
	OrderDesc           string `json:"order_desc"`            // 订单描述
	PaymentAt           int64  `json:"payment_at"`            // 订单支付时间, 毫秒时间戳
	UserRemark          string `json:"user_remark"`           // 用户备注
	ReceiverConfirmAt   int64  `json:"receiver_confirm_at"`   // 确认收货时间
	ReceiverConfirmType int32  `json:"receiver_confirm_type"` // 确认收货方式 1：用户确认收货  99:发货10天自动确认收货
	RefundAmount        int64  `json:"refund_amount"`         // 订单退款金额
	RefundAt            int64  `json:"refund_at"`             // 退款时间，毫秒时间戳
	CancelAt            int64  `json:"cancel_at"`             // 取消时间，毫秒时间戳
	CancelType          int32  `json:"cancel_type"`           // 1: 用户取消  2：客服取消  3: 超时取消
	CancelBy            int64  `json:"cancel_by"`             // 取消人ID， 跟据取消类型判断是用户还是客服， -1为系统超时取消
	CancelReason        string `json:"cancel_reason"`         // 取消理由
	CreateAt            int64  `json:"create_at"`             // 订单创建时间，毫秒时间戳
	CreateBy            int64  `json:"create_by"`             // 订单创建人ID，根据order_source判断是用户，还是客服ID，系统-1
	CancelName          string `json:"cancel_name"`
	CreateName          string `json:"create_name"`
}

type RefundDto struct {
	ID      int64   `json:"id"`
	Amount  int64   `json:"amount"`
	ItemIds []int64 `json:"item_ids"`
	ApplyAt int64   `json:"apply_at"`
	Status  int32   `json:"status"` // 0：无退款，1：退款中，2：退款完成，3：退款异常
	DoneAt  int64   `json:"done_at"`
}

type OrderItemDto struct {
	ID             int64       `json:"id"`
	OrderID        int64       `json:"order_id"`
	UserID         int64       `json:"user_id"`
	GoodsID        int64       `json:"goods_id"`
	GoodsType      int32       `json:"goods_type"`
	Quantity       int32       `json:"quantity"`
	PaymentAmount  int64       `json:"payment_amount"`
	DiscountAmount int64       `json:"discount_amount"`
	GoodsSnap      interface{} `json:"goods_snap"`
	RefundStatus   int32       `json:"refund_status"` // 0：无退款，1：退款中，2：退款完成，3：退款异常
}

type OrderInfoResp struct {
	*OrderDto
	Items   []*OrderItemDto `json:"items"`
	Refunds []*RefundDto    `json:"refunds"`
}

type GetOrderListResp struct {
	common.Pager
	Total int64       `json:"total"`
	List  []*OrderDto `json:"list"`
}

type GetUserOrderListResp struct {
	common.Pager
	Total int64            `json:"total"`
	List  []*OrderInfoResp `json:"list"`
}

type OrderRefundReq struct {
	OrderID int64  `json:"order_id"`
	Reason  string `json:"reason"`
	Amount  int64  `json:"amount"`
}

type OrderStatReq struct {
	common.Pager
	DateType  int32 `url:"date_type"` // 1:按天 2:按月 3：按季度 4:按年
	GoodsID   int64 `url:"goods_id"`
	StartTime int64 `url:"start_time"`
	EndTime   int64 `url:"end_time"`
}

type OrderStatDto struct {
	Date         string `json:"date"`
	OrderCount   int64  `json:"order_count"`
	OrderAmount  int64  `json:"order_amount"`
	PaymentCount int64  `json:"payment_count"`
	RefundAmount int64  `json:"refund_amount"`
	RefundCount  int64  `json:"refund_count"`
}

type OrderStatResp struct {
	common.Pager
	Total int64           `json:"total"`
	List  []*OrderStatDto `json:"list"`
}

type MockPayReq struct {
	OrderID int64 `json:"order_id"`
}
