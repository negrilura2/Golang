package config

import (
	"strings"
	"testing"
)

func hasErr(errs []error, substr string) bool {
	for _, e := range errs {
		if strings.Contains(e.Error(), substr) {
			return true
		}
	}
	return false
}

func TestValidateEmpty(t *testing.T) {
	cases := []struct {
		name     string
		conf     *Config
		wantCnt  int
		contains []string
	}{
		{name: "全空", conf: &Config{}, wantCnt: 9, contains: []string{"mysql.host", "BizConf.MobileSecret长度"}},
		{name: "全对", conf: &Config{
			Server:  Server{Env: "dev"},
			Mysql:   Mysql{Host: "127.0.0.1", Port: 3306, User: "root", Database: "edu.mall"},
			Redis:   Redis{Addr: "127.0.0.1:6379"},
			BizConf: BizConf{MobileSecret: "1234567890123456", CaptchaSecret: "dev"},
		}, wantCnt: 0},
		{name: "密钥长度错", conf: &Config{
			Server:  Server{Env: "dev"},
			Mysql:   Mysql{Host: "127.0.0.1", Port: 3306, User: "root", Database: "edu.mall"},
			Redis:   Redis{Addr: "127.0.0.1:6379"},
			BizConf: BizConf{MobileSecret: "short", CaptchaSecret: "dev"},
		}, wantCnt: 1, contains: []string{"长度"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			errs := c.conf.Validate()
			if len(errs) != c.wantCnt {
				t.Errorf("len(errs) = %d, want %d, errs = %v", len(errs), c.wantCnt, errs)
			}
			for _, sub := range c.contains {
				if !hasErr(errs, sub) {
					t.Errorf("缺少错误消息： %q, 实际: %v", sub, errs)
				}
			}
		})
	}
}
