package admin

import (
	"github.com/wenlng/go-captcha/v2/slide"
	"mall/adaptor"
	"mall/adaptor/redis"
	"mall/adaptor/repo/admin"
	"mall/adaptor/rpc"
	"mall/config"
	"mall/service/token"
	"mall/utils/captcha"
)

type Service struct {
	conf      *config.Config
	adminUser admin.IAdminUser
	adminRole admin.IAdminRole
	verify    redis.IVerify
	captcha   slide.Captcha
	token     *token.Service
	lark      rpc.ILark
}

func NewService(adaptor adaptor.IAdaptor) *Service {
	return &Service{
		conf:      adaptor.GetConfig(),
		adminUser: admin.NewAdminUser(adaptor),
		verify:    redis.NewVerify(adaptor),
		captcha:   captcha.NewSlideCaptcha(),
		token:     token.NewService(adaptor),
		lark:      rpc.NewLark(adaptor),
		adminRole: admin.NewAdminRole(adaptor),
	}
}
