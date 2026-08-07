package rpc

import (
	"context"
	"crypto/rsa"
	"fmt"
	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/wechat/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"mall/adaptor"
	"mall/adaptor/repo/query"
	"mall/config"
	"mall/utils/logger"
	"sync"
)

type IPay interface {
	GetPublicKeyMap() map[string]*rsa.PublicKey
	// jsapi和小程序支付下单
	JsApiPrePayOrder(ctx context.Context, req gopay.BodyMap) (string, *wechat.JSAPIPayParams, error)
	// 通过商户的订单号查询订单
	QueryOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*wechat.QueryOrder, error)
	// 通过支付平台的订单号查询订单
	QueryOrderByTransactionID(ctx context.Context, transactionID string) (*wechat.QueryOrder, error)
	// 关闭订单
	CloseOrder(ctx context.Context, outTradeNo string) error
	// 订单申请退款
	ApplyOrderRefund(ctx context.Context, req gopay.BodyMap) (*wechat.RefundOrderResponse, error)
	// 订单退款查询
	QueryOrderRefund(ctx context.Context, outTradeNo string) (*wechat.RefundQueryResponse, error)
	// TODO 小程序订单，需要上传发货信息
}

var (
	wechatClient *wechat.ClientV3
	once         sync.Once
)

type WechatPay struct {
	conf         *config.Config
	db           *gorm.DB
	wechatClient *wechat.ClientV3
}

func NewWechatPay(adaptor adaptor.IAdaptor) *WechatPay {
	payer := &WechatPay{
		conf: adaptor.GetConfig(),
		db:   adaptor.GetDB(),
	}

	once.Do(func() {
		wxClient, err := payer.initWechatPayClient()
		if err != nil {
			logger.Error("init wechat pay client error", zap.Error(err))
			panic(err)
		}
		wechatClient = wxClient
	})
	payer.wechatClient = wechatClient
	return payer
}

func (a *WechatPay) getPrivateKey() (string, error) {
	qs := query.Use(a.db).PaymentPrivateKey
	first, err := qs.WithContext(context.TODO()).Where(qs.AppID.Eq(a.conf.WechatPay.AppID)).First()
	if err != nil {
		return "", err
	}
	return first.PrivateKey, nil
}

func (a *WechatPay) initWechatPayClient() (*wechat.ClientV3, error) {
	privateKey, err := a.getPrivateKey()
	if err != nil {
		return nil, err
	}
	client, err := wechat.NewClientV3(
		a.conf.WechatPay.MchID,
		a.conf.WechatPay.CertSerialNo,
		a.conf.WechatPay.ApiKey,
		privateKey)
	if err != nil {
		return nil, err
	}

	err = client.AutoVerifySign()
	if err != nil {
		return nil, err
	}
	if !a.conf.WechatPay.IsProd {
		client.DebugSwitch = gopay.DebugOn
	}
	return client, nil
}

func (a *WechatPay) GetPublicKeyMap() map[string]*rsa.PublicKey {
	return a.wechatClient.WxPublicKeyMap()
}

const SuccessCode = 0

// jsapi和小程序支付下单
func (a *WechatPay) JsApiPrePayOrder(ctx context.Context, req gopay.BodyMap) (string, *wechat.JSAPIPayParams, error) {
	wxResp, err := a.wechatClient.V3TransactionJsapi(ctx, req)
	if err != nil {
		return "", nil, err
	}
	if wxResp.Code != SuccessCode {
		return "", nil, fmt.Errorf("%+v", wxResp)
	}
	signParams, err := a.wechatClient.PaySignOfJSAPI(a.conf.WechatPay.AppID, wxResp.Response.PrepayId)
	if err != nil {
		return "", nil, err
	}
	return wxResp.Response.PrepayId, signParams, nil
}

// 通过商户的订单号查询订单
func (a *WechatPay) QueryOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*wechat.QueryOrder, error) {
	wxResp, err := a.wechatClient.V3TransactionQueryOrder(ctx, wechat.OrderNoType(2), outTradeNo)
	if err != nil {
		return nil, err
	}
	if wxResp.Code != SuccessCode {
		return nil, fmt.Errorf("%+v", wxResp)
	}
	return wxResp.Response, nil
}

// 通过支付平台的订单号查询订单
func (a *WechatPay) QueryOrderByTransactionID(ctx context.Context, transactionID string) (*wechat.QueryOrder, error) {
	wxResp, err := a.wechatClient.V3TransactionQueryOrder(ctx, wechat.OrderNoType(1), transactionID)
	if err != nil {
		return nil, err
	}
	if wxResp.Code != SuccessCode {
		return nil, fmt.Errorf("%+v", wxResp)
	}
	return wxResp.Response, nil
}

// 关闭订单
func (a *WechatPay) CloseOrder(ctx context.Context, outTradeNo string) error {
	wxResp, err := a.wechatClient.V3TransactionCloseOrder(ctx, outTradeNo)
	if err != nil {
		return err
	}
	if wxResp.Code != SuccessCode {
		return fmt.Errorf("%+v", wxResp)
	}
	return nil
}

// 订单申请退款
func (a *WechatPay) ApplyOrderRefund(ctx context.Context, req gopay.BodyMap) (*wechat.RefundOrderResponse, error) {
	wxResp, err := a.wechatClient.V3Refund(ctx, req)
	if err != nil {
		return nil, err
	}
	if wxResp.Code != SuccessCode {
		return nil, fmt.Errorf("%+v", wxResp)
	}
	return wxResp.Response, nil
}

// 订单退款查询
func (a *WechatPay) QueryOrderRefund(ctx context.Context, outTradeNo string) (*wechat.RefundQueryResponse, error) {
	wxResp, err := a.wechatClient.V3RefundQuery(ctx, outTradeNo, nil)
	if err != nil {
		return nil, err
	}
	if wxResp.Code != SuccessCode {
		return nil, fmt.Errorf("%+v", wxResp)
	}
	return wxResp.Response, nil
}
