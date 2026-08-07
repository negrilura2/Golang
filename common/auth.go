package common

type AdminUser struct {
	UserID     int64  `json:"user_id"`
	Name       string `json:"name"`
	NickName   string `json:"nick_name"`
	Sex        int32  `json:"sex"`
	Status     int32  `json:"status"`
	Mobile     string `json:"mobile"`
	LarkOpenID string `json:"lark_open_id"`
}

type User struct {
	ID          int64  `json:"id"`
	NickName    string `json:"nick_name"`
	CreateAt    int64  `json:"create_at"`
	IconUrl     string `json:"icon_url"`
	Sex         int32  `json:"sex"`
	Status      int32  `json:"status"`
	LastLoginAt int64  `json:"last_login_at"`
	UpdateAt    int64  `json:"update_at"`
}

type MobileUser struct {
	Mobile string `json:"mobile"`
	UserID int64  `json:"user_id"`
}

type WechatUser struct {
	UserID  int64  `json:"user_id"`
	UnionID string `json:"union_id"`
}

type AppUser struct {
	OpenID  string `json:"open_id"`
	UserID  int64  `json:"user_id"`
	AppCode int32  `json:"app_code"`
}

type UserInfo struct {
	User       User        `json:"user"`
	MobileUser *MobileUser `json:"mobile_user"`
	WechatUser *WechatUser `json:"wechat_user"`
	AppUsers   []AppUser   `json:"app_users"`
}
