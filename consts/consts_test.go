package consts

import "testing"

func TestGetRefundStatus(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int32
	}{
		{name: "微信支付成功", in: "SUCCESS", want: RefundStatusDone},
		{name: "支付关闭", in: "CLOSED", want: RefundStatusDone},
		{name: "支付异常", in: "ABNORMAL", want: RefundStatusException},
		{name: "支付中", in: "PROCESSING", want: RefundStatusProcessing},
		{name: "都行", in: "xwxwxwxw", want: RefundStatusProcessing},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := GetRefundStatus(c.in)
			if got != c.want {
				t.Errorf("GetRefundStatus(%q) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestGetExpireTime(t *testing.T) {
	cases := []struct {
		name  string
		in    int32
		valid bool
	}{
		{name: "有效一个月", in: LearnExpireTimeMonth, valid: true},
		{name: "有效一年", in: LearnExpireTimeYear, valid: true},
		{name: "非法类型", in: 99, valid: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := GetExpireTime(c.in)
			if c.valid {
				if got == 0 {
					t.Errorf("有效类型得到0， 期望非0")
				}
			} else if got != 0 {
				t.Errorf("非法类型 %d = %d, 期望0", c.in, got)
			}
		})
	}
}
