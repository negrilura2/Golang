package rpc

import (
	"context"
	"crypto/rsa"

	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/wechat/v3"
)

// MockPay 是本地开发/学习环境使用的"假支付"实现。
// 它实现 IPay 接口的全部方法，模拟微信支付各接口的返回，
// 使依赖 payment 的订单链路（pay_now / pay_later / cancel / 定时查单）在没有真实商户号时完整走通。
//
// 注意：它不产生任何真实微信支付行为，仅用于本地走通业务链路。
// 接入真实商户号并建好 payment_private_key 表后，将 service/order/service.go 中
// payment 的赋值换回 rpc.NewWechatPay(adaptor) 即可。
type MockPay struct{}

func NewMockPay() *MockPay {
	return &MockPay{}
}

// GetPublicKeyMap 回调验签用，本地无真实平台证书，返回空 map。
func (m *MockPay) GetPublicKeyMap() map[string]*rsa.PublicKey {
	return make(map[string]*rsa.PublicKey)
}

// JsApiPrePayOrder 模拟微信支付下单，返回假的预支付参数。
// 第一个返回值对应真实实现的 PrepayId。
func (m *MockPay) JsApiPrePayOrder(ctx context.Context, req gopay.BodyMap) (string, *wechat.JSAPIPayParams, error) {
	return "mock-prepay-id",
		&wechat.JSAPIPayParams{
			AppId:     "mock-appid",
			TimeStamp: "1",
			NonceStr:  "mock-noncestr",
			Package:   "prepay_id=mock-prepay-id",
			SignType:  "RSA",
			PaySign:   "mock-paysign",
		}, nil
}

// QueryOrderByOutTradeNo 模拟查单，固定返回"未支付"，
// 让主动查单补偿链路保持等待，直到超时取消或用户手动取消。
func (m *MockPay) QueryOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*wechat.QueryOrder, error) {
	return &wechat.QueryOrder{TradeState: "NOTPAY"}, nil
}

func (m *MockPay) QueryOrderByTransactionID(ctx context.Context, transactionID string) (*wechat.QueryOrder, error) {
	return &wechat.QueryOrder{TradeState: "NOTPAY"}, nil
}

// CloseOrder 模拟关闭订单，本地不做任何事。
func (m *MockPay) CloseOrder(ctx context.Context, outTradeNo string) error {
	return nil
}

// ApplyOrderRefund 模拟退款申请，返回空结果表示成功。
func (m *MockPay) ApplyOrderRefund(ctx context.Context, req gopay.BodyMap) (*wechat.RefundOrderResponse, error) {
	return &wechat.RefundOrderResponse{}, nil
}

// QueryOrderRefund 模拟退款查询。
func (m *MockPay) QueryOrderRefund(ctx context.Context, outTradeNo string) (*wechat.RefundQueryResponse, error) {
	return &wechat.RefundQueryResponse{}, nil
}
