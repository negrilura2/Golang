package redis

import (
	"context"
	"github.com/go-redis/redis"
	"mall/consts"
	"testing"
)

func TestCourseCacheLifecycle(t *testing.T) {
	rds := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	// Ping 探活： redis 没起就Skip 而不是Fail
	if err := rds.Ping().Err(); err != nil {
		t.Skipf("redis 未启动， 跳过集成测试: %v", err)
	}

	c := &Course{redis: rds}
	id := int64(999)
	t.Cleanup(func() {
		rds.Del(fmtCourseInfoKey(id))
	})
	if _, err := c.GetCourseInfo(context.Background(), id); err != redis.Nil {
		t.Fatalf("初始应未命中， 实际 err=%v", err)
	}
	setValue := `{"id":999,"name":"测试课程"}`
	if err := c.SetCourseInfo(context.Background(), id, setValue, consts.CourseInfoCacheExpire); err != nil {
		t.Fatalf("SetCourseInfo 失败： %v", err)
	}

	got, err := c.GetCourseInfo(context.Background(), id)
	if err != nil {
		t.Fatalf("Set后应该命中，实际err=%v", err)
	}
	if got != setValue {
		t.Errorf("值=%q， 期望=%q", got, setValue)
	}

	if err := c.DelCourseInfo(context.Background(), id); err != nil {
		t.Fatalf("DelCourseInfo 失败: %v", err)
	}
	if _, err := c.GetCourseInfo(context.Background(), id); err != redis.Nil {
		t.Fatalf("Del 后应该未命中， 实际 err=%v", err)
	}
}
