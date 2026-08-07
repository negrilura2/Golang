package rpc

import (
	"context"
	"fmt"
	"mall/adaptor"
	"mall/config"
	"mall/utils/http"
)

type IWechat interface {
	Code2Session(ctx context.Context, appCode int32, wxCode string) (*Code2SessionResp, error)
	GetAppletAccessToken(ctx context.Context, appCode int32) (*AppletAccessTokenResp, error)
}

type Wechat struct {
	conf *config.Config
}

func NewWechat(adaptor adaptor.IAdaptor) *Wechat {
	return &Wechat{
		conf: adaptor.GetConfig(),
	}
}

type Code2SessionResp struct {
	Openid     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionId    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

func (c *Code2SessionResp) IsOk() bool {
	return c.ErrCode == 0
}

const (
	WechatHost = "https://api.weixin.qq.com"
)

func (w *Wechat) Code2Session(ctx context.Context, appCode int32, wxCode string) (*Code2SessionResp, error) {
	appConf, ok := w.conf.AppConf[appCode]
	if !ok {
		return nil, fmt.Errorf("app_code:%d not found", appCode)
	}
	resp := &Code2SessionResp{}
	url := fmt.Sprintf("%s/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		WechatHost,
		appConf.AppID,
		appConf.AppSecret,
		wxCode)
	err := http.Get(ctx, url, nil, nil, resp)
	if err != nil {
		return nil, err
	}
	if !resp.IsOk() {
		return nil, fmt.Errorf("code2session errcode:%d, errmsg:%s", resp.ErrCode, resp.ErrMsg)
	}
	return resp, nil
}

type AppletAccessTokenResp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

func (c *AppletAccessTokenResp) IsOk() bool {
	return c.ErrCode == 0
}

func (w *Wechat) GetAppletAccessToken(ctx context.Context, appCode int32) (*AppletAccessTokenResp, error) {
	appConf, ok := w.conf.AppConf[appCode]
	if !ok {
		return nil, fmt.Errorf("app_code:%d not found", appCode)
	}
	resp := &AppletAccessTokenResp{}
	url := fmt.Sprintf("%s/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		WechatHost,
		appConf.AppID,
		appConf.AppSecret)
	err := http.Get(ctx, url, nil, nil, resp)
	if err != nil {
		return nil, err
	}
	if !resp.IsOk() {
		return nil, fmt.Errorf("code2session errcode:%d, errmsg:%s", resp.ErrCode, resp.ErrMsg)
	}
	return resp, nil
}
