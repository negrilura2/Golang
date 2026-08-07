package user

import (
	"github.com/wenlng/go-captcha/v2/slide"
	"mall/adaptor"
	"mall/adaptor/redis"
	"mall/adaptor/repo/goods"
	"mall/adaptor/repo/user"
	"mall/adaptor/rpc"
	"mall/config"
	goodsSvc "mall/service/goods"
	"mall/service/token"
	"mall/utils/captcha"
)

type Service struct {
	conf       *config.Config
	verify     redis.IVerify
	captcha    slide.Captcha
	lark       rpc.ILark
	user       user.IUser
	token      *token.Service
	userCourse user.IUserCourse
	goods      goods.ICourse
	wechat     rpc.IWechat
	storage    rpc.IStorage
	goodsSvc   *goodsSvc.Service
}

func NewService(adaptor adaptor.IAdaptor) *Service {
	return &Service{
		conf:       adaptor.GetConfig(),
		verify:     redis.NewVerify(adaptor),
		captcha:    captcha.NewSlideCaptcha(),
		lark:       rpc.NewLark(adaptor),
		token:      token.NewService(adaptor),
		user:       user.NewUser(adaptor),
		userCourse: user.NewUserCourse(adaptor),
		goods:      goods.NewCourse(adaptor),
		wechat:     rpc.NewWechat(adaptor),
		storage:    rpc.NewStorage(adaptor),
		goodsSvc:   goodsSvc.NewService(adaptor),
	}
}
