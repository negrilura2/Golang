package main

// 临时开发工具：查看/播种登录演示数据（演示完删除）
// 用法：
//   go run ./tools/dbseed list
//   go run ./tools/dbseed seed-admin <mobile> <password> [plain]   # 默认 bcrypt，加 plain 存明文
//   go run ./tools/dbseed seed-user  <mobile> <password> [plain]   # 默认 bcrypt，加 plain 存明文
//   go run ./tools/dbseed errcount <mobile>                        # 看两端错误计数 key

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"mall/adaptor/repo/model"
	"mall/utils/password"
	"mall/utils/tools"
)

const (
	dsn          = "root:123456@tcp(127.0.0.1:3306)/edu.mall?charset=utf8mb4&parseTime=true&loc=Local"
	mobileSecret = "mall-demo-secret" // 必须与 mall_local.yml 的 mobile_secret 一致，且为 16/24/32 字节
	serverPrefix = "edu.mall"         // 必须与 config.ServerFullName 一致
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "list":
		list()
	case "seed-admin":
		if len(os.Args) < 4 {
			usage()
			os.Exit(1)
		}
		seedAdmin(os.Args[2], os.Args[3], hasPlainFlag(os.Args[4:]))
	case "seed-user":
		if len(os.Args) < 4 {
			usage()
			os.Exit(1)
		}
		seedUser(os.Args[2], os.Args[3], hasPlainFlag(os.Args[4:]))
	case "errcount":
		if len(os.Args) < 3 {
			usage()
			os.Exit(1)
		}
		errcount(os.Args[2])
	case "seed-app":
		if len(os.Args) < 5 {
			usage()
			os.Exit(1)
		}
		seedApp(os.Args[2], os.Args[3], os.Args[4])
	default:
		usage()
		os.Exit(1)
	}
}

func hasPlainFlag(rest []string) bool {
	for _, a := range rest {
		if a == "plain" {
			return true
		}
	}
	return false
}

func usage() {
	fmt.Println("用法: go run ./tools/dbseed <list|seed-admin|seed-user|errcount> ...")
}

func openDB() *gorm.DB {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func desc(s string) string {
	if s == "" {
		return "(空)"
	}
	if strings.HasPrefix(s, "$2") {
		return "bcrypt:" + s[:min(31, len(s))] + "..."
	}
	return "明文:" + s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func seedAdmin(mobile, pwd string, plain bool) {
	db := openDB()
	stored := pwd
	if !plain {
		h, err := password.HashPassword(pwd)
		if err != nil {
			log.Fatal(err)
		}
		stored = h
	}
	now := time.Now()
	var existing model.AdminUser
	err := db.Where("mobile = ?", mobile).First(&existing).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		admin := model.AdminUser{
			Name: "demo", NickName: "演示管理员", Mobile: mobile, Password: stored,
			Status: 1, Sex: 3, CreateAt: now, UpdateAt: now, CreateBy: 1, UpdateBy: 1, IsDelete: 0,
		}
		if err := db.Create(&admin).Error; err != nil {
			log.Fatal(err)
		}
		fmt.Printf("创建 admin_user: id=%d mobile=%s password=%s\n", admin.ID, mobile, desc(stored))
	case err == nil:
		if err := db.Model(&model.AdminUser{}).Where("mobile = ?", mobile).
			Updates(map[string]interface{}{"password": stored, "status": 1, "update_at": now}).Error; err != nil {
			log.Fatal(err)
		}
		fmt.Printf("更新 admin_user: id=%d mobile=%s password=%s\n", existing.ID, mobile, desc(stored))
	default:
		log.Fatal(err)
	}
}

func seedUser(mobile, pwd string, plain bool) {
	db := openDB()
	stored := pwd
	if !plain {
		h, err := password.HashPassword(pwd)
		if err != nil {
			log.Fatal(err)
		}
		stored = h
	}
	mobileSha := tools.Sha256Hash(mobile)
	mobileAes, err := tools.AESEncrypt(mobile, []byte(mobileSecret))
	if err != nil {
		log.Fatal(err)
	}
	now := time.Now()

	var mu model.MobileUser
	err = db.Where("mobile_sha256 = ?", mobileSha).First(&mu).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		u := model.User{
			NickName: "演示用户" + mobile[len(mobile)-4:], Password: stored,
			Status: 1, CreateAt: now, LastLoginAt: now, UpdateAt: now,
		}
		if err := db.Create(&u).Error; err != nil {
			log.Fatal(err)
		}
		mu = model.MobileUser{UserID: u.ID, MobileAes: mobileAes, MobileSha256: mobileSha, CreateAt: now, UpdateAt: now}
		if err := db.Create(&mu).Error; err != nil {
			log.Fatal(err)
		}
		fmt.Printf("创建 user(id=%d)+mobile_user(id=%d): mobile=%s password=%s\n", u.ID, mu.ID, mobile, desc(stored))
	case err == nil:
		if err := db.Model(&model.User{}).Where("id = ?", mu.UserID).
			Updates(map[string]interface{}{"password": stored, "status": 1, "update_at": now}).Error; err != nil {
			log.Fatal(err)
		}
		if err := db.Model(&model.MobileUser{}).Where("id = ?", mu.ID).
			Updates(map[string]interface{}{"mobile_aes": mobileAes, "update_at": now}).Error; err != nil {
			log.Fatal(err)
		}
		fmt.Printf("更新 user(id=%d): mobile=%s password=%s\n", mu.UserID, mobile, desc(stored))
	default:
		log.Fatal(err)
	}
}

func list() {
	db := openDB()
	fmt.Println("=== admin_user ===")
	var admins []model.AdminUser
	db.Order("id").Find(&admins)
	for _, a := range admins {
		fmt.Printf("id=%d mobile=%s status=%d password=%s\n", a.ID, a.Mobile, a.Status, desc(a.Password))
	}
	fmt.Println("=== user ===")
	var users []model.User
	db.Order("id").Find(&users)
	for _, u := range users {
		fmt.Printf("id=%d nick=%s status=%d password=%s\n", u.ID, u.NickName, u.Status, desc(u.Password))
	}
	fmt.Println("=== mobile_user ===")
	var mus []model.MobileUser
	db.Order("id").Find(&mus)
	for _, m := range mus {
		fmt.Printf("id=%d user_id=%d mobile_sha256=%s mobile_aes=%s\n", m.ID, m.UserID, m.MobileSha256, m.MobileAes)
	}
}

func errcount(mobile string) {
	rds := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer rds.Close()
	scenes := []struct {
		name string
		v    int
	}{{"admin(场景1)", 1}, {"customer(场景2)", 2}}
	for _, s := range scenes {
		key := fmt.Sprintf("%s:%d:user:password:errcount:%s", serverPrefix, s.v, mobile)
		v, _ := rds.Get(key).Int64()
		ttl, _ := rds.TTL(key).Result()
		fmt.Printf("  %-14s errcount=%d  ttl=%v\n  key=%s\n", s.name, v, ttl, key)
	}
}

func seedApp(userID, appCode, openID string) {
	db := openDB()
	uid, err := strconv.Atoi(userID)
	if err != nil {
		log.Fatal(err)
	}
	code, err := strconv.Atoi(appCode)
	if err != nil {
		log.Fatal(err)
	}
	app := model.AppUser{
		UserID:   int64(uid),
		AppCode:  int32(code),
		OpenID:   openID,
		Status:   1,
		CreateAt: time.Now(),
		UpdateAt: time.Now(),
	}
	if err := db.Create(&app).Error; err != nil {
		log.Fatal(err)
	}
	fmt.Printf("创建app_user: user_id=%d app_code=%d open_id=%s\n", uid, code, openID)
}
