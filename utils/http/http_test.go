package http

import (
	"context"
	"testing"
)

func TestGet(t *testing.T) {
	type args struct {
		ctx    context.Context
		reqUrl string
		header map[string]string
		data   map[string]interface{}
		out    interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "test",
			args: args{
				ctx:    context.Background(),
				reqUrl: "https://www.baidu.com/sugrec?&prod=pc_his&from=pc_web&json=1&sid=63144_65361_65986_66121_65866_66147_66207_66228_66235_66362_66381_66273_66261_66393_66394_66518_66529_66554_66587_66590_66521_65802&hisdata=%5B%7B%22time%22%3A1745747553%2C%22kw%22%3A%22windows11%E4%B8%93%E4%B8%9A%E7%89%88%E6%BF%80%E6%B4%BB%E5%B7%A5%E5%85%B7%22%2C%22fq%22%3A2%7D%2C%7B%22time%22%3A1747314732%2C%22kw%22%3A%22%E7%8C%AB%E7%9A%84%E5%9B%BE%E7%89%87%22%7D%2C%7B%22time%22%3A1749391221%2C%22kw%22%3A%22http%3A%2F%2Ffish.audio%2F%22%7D%2C%7B%22time%22%3A1749391230%2C%22kw%22%3A%22fish.audio%2F%22%7D%2C%7B%22time%22%3A1749472262%2C%22kw%22%3A%22%E5%A6%82%E4%BD%95%E4%BD%BF%E7%94%A8ai%E5%B0%86%E9%9F%B3%E4%B9%90%E7%9A%84%E5%A3%B0%E9%9F%B3%E6%9B%B4%E6%8D%A2%E6%88%90%E8%87%AA%E5%B7%B1%E7%9A%84%22%7D%2C%7B%22time%22%3A1749472323%2C%22kw%22%3A%22%E4%BD%BF%E7%94%A8fish%20audio%22%7D%2C%7B%22time%22%3A1749649841%2C%22kw%22%3A%22%E4%BD%BF%E7%94%A8fish%20audio%20%E6%9B%B4%E6%8D%A2%E9%9F%B3%E4%B9%90%E7%9A%84%E5%A3%B0%E9%9F%B3%22%2C%22fq%22%3A2%7D%2C%7B%22time%22%3A1749650745%2C%22kw%22%3A%22mt87%22%7D%2C%7B%22time%22%3A1749650749%2C%22kw%22%3A%22%E5%89%8D%E8%A1%8C%E8%80%85mt87%E9%94%AE%E7%9B%98%E8%AF%B4%E6%98%8E%E4%B9%A6%22%7D%2C%7B%22time%22%3A1762651664%2C%22kw%22%3A%22kd3sd.mingri.icu%20%E8%8B%8F%E6%89%93%E4%BA%91%E6%A2%AF%E5%AD%90%E5%A4%B1%E6%95%88%E4%BA%86%22%7D%5D&_t=1763820931269&req=2&csor=0",
				header: map[string]string{
					"Accept": "application/json",
				},
				data: map[string]interface{}{
					"1": 2,
				},
				out: make([]int64, 0),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Get(tt.args.ctx, tt.args.reqUrl, tt.args.header, tt.args.data, tt.args.out); (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
