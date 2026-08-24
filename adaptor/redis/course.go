package redis

import (
	"context"
	"fmt"
	"github.com/go-redis/redis"
	"mall/adaptor"
	"mall/config"
	"time"
)

type ICourse interface {
	GetCourseInfo(ctx context.Context, id int64) (string, error)
	SetCourseInfo(ctx context.Context, id int64, info string, expire time.Duration) error
	DelCourseInfo(ctx context.Context, id int64) error
}
type Course struct {
	redis *redis.Client
}

func NewCourse(adaptor adaptor.IAdaptor) *Course {
	return &Course{
		redis: adaptor.GetRedis(),
	}
}
func fmtCourseInfoKey(id int64) string {
	return fmt.Sprintf("%s:course:info:%d", config.ServerFullName, id)
}
func (c *Course) GetCourseInfo(ctx context.Context, id int64) (string, error) {
	return c.redis.Get(fmtCourseInfoKey(id)).Result()
}

func (c *Course) SetCourseInfo(ctx context.Context, id int64, info string, expire time.Duration) error {
	redisKey := fmtCourseInfoKey(id)
	return c.redis.Set(redisKey, info, expire).Err()
}

func (c *Course) DelCourseInfo(ctx context.Context, id int64) error {
	redisKey := fmtCourseInfoKey(id)
	return c.redis.Del(redisKey).Err()
}
