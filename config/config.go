package config

import (
	"errors"
	"flag"
	"fmt"
	"github.com/go-viper/mapstructure/v2"
	"github.com/gogf/gf/util/gconv"
	"github.com/spf13/viper"
	_ "github.com/spf13/viper/remote"
	"gopkg.in/yaml.v3"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	ServerName     = "mall"
	ServerFullName = "edu.mall"
)

var (
	etcdKey         = fmt.Sprintf("/configs/%s/system", ServerFullName)
	etcdAddr        string
	localConfigPath string
	GlobalConfig    Config
)

type Config struct {
	Server    Server            `yaml:"server"`
	Mysql     Mysql             `yaml:"mysql"`
	Redis     Redis             `yaml:"redis"`
	AppConf   map[int32]AppConf `yaml:"app_conf"`
	BizConf   BizConf           `yaml:"biz_conf"`
	Storage   Storage           `yaml:"storage"`
	WechatPay WechatPay         `yaml:"wechat_pay"`
	Kafka     Kafka             `yaml:"kafka"`
}

type Server struct {
	HttpPort       int      `yaml:"http_port"`
	Env            string   `yaml:"env"`
	EnablePprof    bool     `yaml:"enable_pprof"`
	LogLevel       string   `yaml:"log_level"`
	TrustedProxies []string `yaml:"trusted_proxies"` //新增： 信任的反代来源，支持 CIDR
}
type Kafka struct {
	Brokers []string `yaml:"brokers"`
}
type Mysql struct {
	Dialect  string `yaml:"dialect"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	Charset  string `yaml:"charset"`
	ShowSql  bool   `yaml:"show_sql"`
	MaxOpen  int    `yaml:"max_open"`
	MaxIdle  int    `yaml:"max_idle"`
}

type Storage struct {
	SecretID  string `yaml:"secret_id"`
	SecretKey string `yaml:"secret_key"`
	AppID     string `yaml:"app_id"`
	Bucket    Bucket `yaml:"bucket"`
}

type Bucket struct {
	Region     string            `yaml:"region"`
	BucketName string            `yaml:"bucket_name"`
	Domain     string            `yaml:"domain"`
	CdnDomain  string            `yaml:"cdn_domain"`
	Paths      map[string]string `yaml:"paths"`
}

func (m *Mysql) GetDsn() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&loc=Local",
		m.User, m.Password, m.Host, m.Port, m.Database, m.Charset)
}

type AppConf struct {
	AppType   string `yaml:"app_type"`
	AppName   string `yaml:"app_name"`
	AppID     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
}

type BizConf struct {
	LarkGroupID   string `yaml:"lark_group_id"`
	MobileSecret  string `yaml:"mobile_secret"`
	BizSecret     string `yaml:"biz_secret"`
	CaptchaSecret string `yaml:"captcha_secret"`
}

type Redis struct {
	Addr    string `yaml:"addr"`
	PWD     string `yaml:"password"`
	DBIndex int    `yaml:"db_index"`
	MaxIdle int    `yaml:"max_idle"`
	MaxOpen int    `yaml:"max_open"`
}

type WechatPay struct {
	AppID        string `yaml:"app_id"`
	MchID        string `yaml:"mch_id"`
	ApiKey       string `yaml:"api_key"`
	CertSerialNo string `yaml:"cert_serial_no"`
	CallbackUrl  string `yaml:"callback_url"` // 1>回调，2>定时查询（每10秒钟）
	IsProd       bool   `yaml:"is_prod"`      // 是否生产环境
}

type WechatApp struct {
	AppName   string `yaml:"app_name"`
	AppID     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
}

func init() {
	flag.StringVar(&localConfigPath, "c", ServerName+"_local.yml", "default config path")
	flag.StringVar(&etcdAddr, "r", os.Getenv("ETCD_ADDR"), "default etcd address")
}

func InitConfig() *Config {
	var (
		err      error
		tempConf = &Config{}
		vipConf  = viper.New()
	)
	vipConf.SetConfigType("yaml")
	flag.Parse()

	// etcd地址存在，优先使用etcd的配置
	if etcdAddr != "" {
		tempConf, err = getFromRemoteAndWatchUpdate(vipConf)
		if err != nil {
			panic(err)
		}
		overrideFromEnv(tempConf)
		return tempConf
	}

	// 从本地获取
	tempConf, err = getFromLocal()
	if err != nil {
		panic(err)
	}
	overrideFromEnv(tempConf)
	if tempConf.Mysql.Host != "" && tempConf.Mysql.Charset == "" {
		tempConf.Mysql.Charset = "utf8mb4"
	}
	if tempConf.Mysql.Host != "" && tempConf.Mysql.Dialect == "" {
		tempConf.Mysql.Dialect = "mysql"
	}
	return tempConf
}

func getFromRemoteAndWatchUpdate(v *viper.Viper) (*Config, error) {
	tempConf := Config{}
	if err := v.AddRemoteProvider("etcd3", etcdAddr, etcdKey); err != nil {
		return nil, err
	}
	if err := v.ReadRemoteConfig(); err != nil {
		return nil, err
	}

	// 反序列化到结构体
	err := v.Unmarshal(&tempConf, func(config *mapstructure.DecoderConfig) {
		config.TagName = "yaml"
	})

	if err != nil {
		return nil, err
	}

	go func() {
		for {
			time.Sleep(time.Minute * 1)
			if err := v.WatchRemoteConfig(); err == nil {
				_ = v.Unmarshal(&GlobalConfig)
				fmt.Println(">>> etcd config hot-reloaded: ", gconv.String(GlobalConfig))
			}
		}
	}()
	return &tempConf, nil
}

func getFromLocal() (*Config, error) {
	tempConf := Config{}
	content, err := os.ReadFile(localConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			//纯env模式，没配置文件很正常，交给环境变量填充
			return &tempConf, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(content, &tempConf); err != nil {
		return nil, err
	}
	return &tempConf, nil
}
func (c *Config) Validate() []error {
	var errs []error

	if c.Mysql.Host == "" {
		errs = append(errs, errors.New("mysql.host 不能为空"))
	}
	if c.Mysql.Port == 0 {
		errs = append(errs, errors.New("mysql.port 不能为空"))
	}
	if c.Mysql.User == "" {
		errs = append(errs, errors.New("mysql.User 不能为空"))
	}
	if c.Mysql.Database == "" {
		errs = append(errs, errors.New("mysql.Database 不能为空"))
	}
	if c.Redis.Addr == "" {
		errs = append(errs, errors.New("redis.Addr 不能为空"))
	}
	if c.BizConf.MobileSecret == "" {
		errs = append(errs, errors.New("BizConf.MobileSecret 不能为空"))
	}
	n := len(c.BizConf.MobileSecret)
	if n != 16 && n != 24 && n != 32 {
		errs = append(errs, errors.New("BizConf.MobileSecret长度= "+strconv.Itoa(n)+", 不为16/24/32"))
	}
	if c.Server.Env == "" {
		errs = append(errs, errors.New("Server.Env 不能为空"))
	}
	if c.BizConf.CaptchaSecret == "" {
		errs = append(errs, errors.New("BizConf.CaptchaSecret 不能为空"))
	}
	return errs
}
func overrideFromEnv(c *Config) {
	if v := os.Getenv("MALL_MOBILE_SECRET"); v != "" {
		c.BizConf.MobileSecret = v
	}
	if v := os.Getenv("MALL_CAPTCHA_SECRET"); v != "" {
		c.BizConf.CaptchaSecret = v
	}
	if v := os.Getenv("MALL_MYSQL_PASSWORD"); v != "" {
		c.Mysql.Password = v
	}
	if v := os.Getenv("MALL_BIZ_SECRET"); v != "" {
		c.BizConf.BizSecret = v
	}
	if v := os.Getenv("MALL_LARK_GROUP_ID"); v != "" {
		c.BizConf.LarkGroupID = v
	}
	if v := os.Getenv("MALL_WECHAT_API_KEY"); v != "" {
		c.WechatPay.ApiKey = v
	}
	if v := os.Getenv("MALL_STORAGE_SECRET_ID"); v != "" {
		c.Storage.SecretID = v
	}
	if v := os.Getenv("MALL_STORAGE_SECRET_KEY"); v != "" {
		c.Storage.SecretKey = v
	}
	if v := os.Getenv("MALL_SERVER_HTTP_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Server.HttpPort = n
		}
	}
	if v := os.Getenv("MALL_SERVER_ENV"); v != "" {
		c.Server.Env = v
	}
	if v := os.Getenv("MALL_SERVER_LOG_LEVEL"); v != "" {
		c.Server.LogLevel = v
	}
	if v := os.Getenv("MALL_MYSQL_HOST"); v != "" {
		c.Mysql.Host = v
	}
	if v := os.Getenv("MALL_MYSQL_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Mysql.Port = n
		}
	}
	if v := os.Getenv("MALL_MYSQL_USER"); v != "" {
		c.Mysql.User = v
	}
	if v := os.Getenv("MALL_MYSQL_PASSWORD"); v != "" {
		c.Mysql.Password = v
	}
	if v := os.Getenv("MALL_MYSQL_DATABASE"); v != "" {
		c.Mysql.Database = v
	}
	if v := os.Getenv("MALL_MYSQL_SHOW_SQL"); v != "" {
		if n, err := strconv.ParseBool(v); err == nil {
			c.Mysql.ShowSql = n
		}
	}
	if v := os.Getenv("MALL_REDIS_ADDR"); v != "" {
		c.Redis.Addr = v
	}
	if v := os.Getenv("MALL_REDIS_PASSWORD"); v != "" {
		c.Redis.PWD = v
	}
	if v := os.Getenv("MALL_REDIS_DB_INDEX"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Redis.DBIndex = n
		}
	}
	if v := os.Getenv("MALL_KAFKA_BROKERS"); v != "" {
		parts := strings.Split(v, ",")
		brokers := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				brokers = append(brokers, p)
			}
		}
		c.Kafka.Brokers = brokers
	}
	if v := os.Getenv("MALL_SERVER_ENABLE_PPROF"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			c.Server.EnablePprof = b
		}
	}
	if v := os.Getenv("MALL_SERVER_TRUSTED_PROXIES"); v != "" {
		parts := strings.Split(v, ",")
		proxies := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				proxies = append(proxies, p)
			}
		}
		c.Server.TrustedProxies = proxies
	}
}
