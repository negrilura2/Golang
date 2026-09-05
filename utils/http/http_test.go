package http

import (
	"context"
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
)

func TestGet(t *testing.T) {
	// httptest 在127.0.0.1 随机端口起一个真HTTP服务器 -- 假“百度”
	srv := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		//断言1：Get里data参数确实拼到了query上
		if got := r.URL.Query().Get("1"); got != "2" {
			t.Errorf("query 1 = %q, want %q", got, "2")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]int64{1, 2, 3})
	}))
	defer srv.Close()
	out := make([]int64, 0)
	err := Get(context.Background(), srv.URL, nil, map[string]interface{}{"1": 2}, &out)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	//断言2：响应被正确反序列化进out
	if len(out) != 3 || out[0] != 1 || out[2] != 3 {
		t.Errorf("out = %v, want[1 2 3]", out)
	}
}
