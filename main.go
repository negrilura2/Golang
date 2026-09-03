package main

import (
	"context"
	"errors"
	"github.com/go-redis/redis"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"mall/adaptor"
	"mall/adaptor/kafka"
	"mall/config"
	"mall/consts"
	"mall/router"
	"mall/service/order"
	"mall/timer"
	"mall/utils/logger"
	"sync"
)

func main() {
	conf := config.InitConfig()
	logger.SetLevel(conf.Server.LogLevel)
	if errs := conf.Validate(); len(errs) > 0 {
		panic(errors.Join(errs...))
	}
	dbClient, err := initMysql(&conf.Mysql)
	handleErr(err)
	logger.Debug("mysql connect success")

	rdsClient, err := initRedis(&conf.Redis)
	handleErr(err)
	logger.Debug("client connect success")

	startServer(conf, dbClient, rdsClient)
}

func startServer(conf *config.Config, db *gorm.DB, redis *redis.Client) {
	newAdaptor := adaptor.NewAdaptor(conf, db, redis)
	app := router.NewApp(conf.Server.HttpPort,
		router.NewRouter(
			conf,
			newAdaptor,
			func() error {
				err := func() error {
					pingDb, err := db.DB()
					handleErr(err)
					return pingDb.Ping()
				}()
				if err != nil {
					return errors.New("mysql connect failed")
				}
				return redis.Ping().Err()
			},
		),
	)

	// 定时器启动
	timerService := timer.NewService(newAdaptor)
	timerService.Start()
	defer timerService.Stop()
	//支付成功事件消费者： 异步在goroutine里跑，kafka取到消息就发权益
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	defer consumerCancel()
	orderSvc := order.NewService(newAdaptor) //消费端用的一套order服务
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := kafka.RunConsumer(consumerCtx,
			conf.Kafka.Brokers,
			consts.KafkaTopicOrderPayed,
			consts.KafkaGroupOrderBenefit,
			orderSvc.HandleOrderPayedEvent); err != nil {
			logger.Error("kafka consumer stopped with error", zap.Error(err))
		}
	}()
	// app启动
	app.Run() // 阻塞到收到 Ctrl+C / SIGTERM，内部先优雅关 HTTP

	//=====优雅停机： 先停接新活的 - 让消费者把手上的消息处理完 - 定时器/生产者最后关 ====
	consumerCancel() // 先通知消费者 “别拉新消息了”
	wg.Wait()        // 等它真退完（把正在处理的那条处理完/提交掉）
}

func initRedis(conf *config.Redis) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         conf.Addr,
		Password:     conf.PWD,
		DB:           conf.DBIndex,
		MinIdleConns: conf.MaxIdle,
		PoolSize:     conf.MaxOpen,
	})
	if r, _ := client.Ping().Result(); r != "PONG" {
		return nil, errors.New("redis connect failed")
	}
	return client, nil
}

func initMysql(conf *config.Mysql) (*gorm.DB, error) {
	conf.MaxIdle = lo.Max([]int{conf.MaxIdle + 1, 5})
	conf.MaxOpen = lo.Max([]int{conf.MaxOpen + 1, 10})
	dsn := conf.GetDsn()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.NewGormLogger(conf.ShowSql)})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if err = sqlDB.Ping(); err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(conf.MaxIdle)
	sqlDB.SetMaxOpenConns(conf.MaxOpen)
	return db, nil
}

func handleErr(err error) {
	if err != nil {
		panic(err)
	}
}
