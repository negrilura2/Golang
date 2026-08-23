package redis

import (
	"context"
	"fmt"
	"github.com/go-redis/redis"
	"mall/adaptor"
	"mall/config"
	"time"
)

type IVerify interface {
	SetCaptchaKey(ctx context.Context, key string, value string, expire time.Duration) error
	GetCaptchaKey(ctx context.Context, key string) (string, error)
	SetCaptchaTicket(ctx context.Context, ticket string, value string, expire time.Duration) error
	GetCaptchaTicket(ctx context.Context, ticket string) (string, error)

	SetVerifyCode(ctx context.Context, mobile string, sceneCode string, value interface{}, expire time.Duration) error
	GetVerifyCode(ctx context.Context, mobile string, sceneCode string) (string, error)
	DelVerifyCode(ctx context.Context, mobile string, sceneCode string)

	SetAdminUserToken(ctx context.Context, userId int64, token string, tokenData string, expire time.Duration) error
	GetAdminUserToken(ctx context.Context, token string) (string, error)
	CleanAdminUserToken(ctx context.Context, userId int64) error

	SetUserToken(ctx context.Context, userId int64, token string, tokenData string, expire time.Duration) error
	GetUserToken(ctx context.Context, token string) (string, error)
	CleanUserToken(ctx context.Context, userId int64) error

	IncrPasswordErr(ctx context.Context, scene int, mobile string, expire time.Duration) (int64, error)
	DeletePasswordErr(ctx context.Context, scene int, mobile string) error
}

type Verify struct {
	redis *redis.Client
}

func NewVerify(adaptor adaptor.IAdaptor) *Verify {
	return &Verify{
		redis: adaptor.GetRedis(),
	}
}

func fmtVerifyCaptchaKey(key string) string {
	return fmt.Sprintf("%s:captcha:%s", config.ServerFullName, key)
}

func fmtVerifyCaptchaTicket(key string) string {
	return fmt.Sprintf("%s:captcha:ticket:%s", config.ServerFullName, key)
}
func (v *Verify) SetCaptchaKey(ctx context.Context, key string, value string, expire time.Duration) error {
	redisKey := fmtVerifyCaptchaKey(key)
	return v.redis.Set(redisKey, value, expire).Err()
}
func (v *Verify) GetCaptchaKey(ctx context.Context, key string) (string, error) {
	redisKey := fmtVerifyCaptchaKey(key)
	get, err := v.redis.Get(redisKey).Result()
	if err != nil {
		return "", err
	}
	v.redis.Del(redisKey)
	return get, nil
}
func (v *Verify) SetCaptchaTicket(ctx context.Context, ticket string, value string, expire time.Duration) error {
	redisKey := fmtVerifyCaptchaTicket(ticket)
	return v.redis.Set(redisKey, value, expire).Err()
}
func (v *Verify) GetCaptchaTicket(ctx context.Context, ticket string) (string, error) {
	redisKey := fmtVerifyCaptchaTicket(ticket)
	get, err := v.redis.Get(redisKey).Result()
	if err != nil {
		return "", err
	}
	v.redis.Del(redisKey)
	return get, nil
}

func fmtVerifyVerifyCode(mobile string, sceneCode string) string {
	return fmt.Sprintf("%s:verify:code:%s:%s", config.ServerFullName, mobile, sceneCode)
}

func (v *Verify) SetVerifyCode(ctx context.Context, mobile string, sceneCode string, value interface{}, expire time.Duration) error {
	redisKey := fmtVerifyVerifyCode(mobile, sceneCode)
	return v.redis.Set(redisKey, value, expire).Err()
}
func (v *Verify) GetVerifyCode(ctx context.Context, mobile string, sceneCode string) (string, error) {
	redisKey := fmtVerifyVerifyCode(mobile, sceneCode)
	return v.redis.Get(redisKey).Result()
}
func (v *Verify) DelVerifyCode(ctx context.Context, mobile string, sceneCode string) {
	redisKey := fmtVerifyVerifyCode(mobile, sceneCode)
	_ = v.redis.Del(redisKey)
}

// -------- 管理后台用户token ---------//
func fmtVerifyAdminUserToken(token string) string {
	return fmt.Sprintf("%s:admin:user:token:%s", config.ServerFullName, token)
}

func fmtUserMapTokenAdminUser(userId int64) string {
	return fmt.Sprintf("%s:admin:token:user:%d", config.ServerFullName, userId)
}
func (v *Verify) SetAdminUserToken(ctx context.Context, userID int64, token string, tokenData string, expire time.Duration) error {
	redisKey := fmtVerifyAdminUserToken(token)
	_, err := v.redis.Set(redisKey, tokenData, expire).Result()
	if err != nil {
		return err
	}
	userMapTokenKey := fmtUserMapTokenAdminUser(userID)
	return v.redis.Set(userMapTokenKey, token, expire).Err()
}
func (v *Verify) GetAdminUserToken(ctx context.Context, token string) (string, error) {
	redisKey := fmtVerifyAdminUserToken(token)
	get, err := v.redis.Get(redisKey).Result()
	if err != nil {
		return "", err
	}
	return get, nil
}

func (v *Verify) CleanAdminUserToken(ctx context.Context, userId int64) error {
	userMapTokenKey := fmtUserMapTokenAdminUser(userId)
	token, err := v.redis.Get(userMapTokenKey).Result()
	if err != nil {
		return err
	}
	redisKey := fmtVerifyAdminUserToken(token)
	return v.redis.Del(redisKey, userMapTokenKey).Err()
}

// -------- C端用户token ---------//
func fmtVerifyUserToken(token string) string {
	return fmt.Sprintf("%s:user:token:%s", config.ServerFullName, token)
}

func fmtUserMapTokenUser(userId int64) string {
	return fmt.Sprintf("%s:token:user:%d", config.ServerFullName, userId)
}
func (v *Verify) SetUserToken(ctx context.Context, userID int64, token string, tokenData string, expire time.Duration) error {
	redisKey := fmtVerifyUserToken(token)
	_, err := v.redis.Set(redisKey, tokenData, expire).Result()
	if err != nil {
		return err
	}
	userMapTokenKey := fmtUserMapTokenUser(userID)
	return v.redis.Set(userMapTokenKey, token, expire).Err()
}
func (v *Verify) GetUserToken(ctx context.Context, token string) (string, error) {
	redisKey := fmtVerifyUserToken(token)
	get, err := v.redis.Get(redisKey).Result()
	if err != nil {
		return "", err
	}
	return get, nil
}

func (v *Verify) CleanUserToken(ctx context.Context, userId int64) error {
	userMapTokenKey := fmtUserMapTokenUser(userId)
	token, err := v.redis.Get(userMapTokenKey).Result()
	if err != nil {
		return err
	}
	redisKey := fmtVerifyUserToken(token)
	return v.redis.Del(redisKey, userMapTokenKey).Err()
}

func fmtVerifyPasswordErr(scene int, mobile string) string {
	return fmt.Sprintf("%s:%d:user:password:errcount:%s", config.ServerFullName, scene, mobile)
}

func (v *Verify) IncrPasswordErr(ctx context.Context, scene int, mobile string, expire time.Duration) (int64, error) {
	redisKey := fmtVerifyPasswordErr(scene, mobile)
	incr, err := v.redis.Incr(redisKey).Result()
	if err != nil {
		return 0, err
	}
	if incr == 1 {
		v.redis.Expire(redisKey, expire)
	}
	return incr, err
}
func (v *Verify) DeletePasswordErr(ctx context.Context, scene int, mobile string) error {
	redisKey := fmtVerifyPasswordErr(scene, mobile)
	return v.redis.Del(redisKey).Err()
}
