package redis

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/gogf/gf/util/gconv"
	"go.uber.org/zap"
	"mall/adaptor"
	"mall/config"
	"mall/utils/logger"
	"time"
)

type IOrder interface {
	// 订单下单前的金额计算
	SetOrderCalcFee(ctx context.Context, feeUUID string, feeData string, expire time.Duration) error
	GetOrderCalcFee(ctx context.Context, feeUUID string) (string, error)
	GetAndDelOrderCalcFee(ctx context.Context, feeUUID string) (string, error)

	// 订单分布式锁
	GetOrderLock(ctx context.Context, orderID int64, uuid string) (bool, error)
	UnLockOrder(ctx context.Context, orderID int64, uuid string) error

	// 订单超时取消
	SetTimeoutOrderCancel(ctx context.Context, orderID int64) error
	GetTimeWaitOrderCancel(ctx context.Context) ([]int64, error)
	DelTimeoutOrderCancel(ctx context.Context, orderID int64) error

	// 订单支付结果轮询
	SetOrderPayResult(ctx context.Context, orderID int64) error
	GetOrderPayResult(ctx context.Context) (map[int64]int64, error)
	DelOrderPayResult(ctx context.Context, orderID int64) error
	CheckInPayment(ctx context.Context, orderID int64) (bool, error)

	// 订单退款结果查询
	SetOrderRefundResult(ctx context.Context, orderID int64) error
	GetOrderRefundResult(ctx context.Context) (map[int64]int64, error)
	DelOrderRefundResult(ctx context.Context, orderID int64) error

	RenewOrderLockLoop(ctx context.Context, orderID int64, uuid string, renewInterval, lockTTL time.Duration) (stop func(), err error)
}

func fmtOrderCalcFeeKey(feeUUID string) string {
	return fmt.Sprintf("%s:order:calc:fee:%s", config.ServerFullName, feeUUID)
}

type Order struct {
	redis *redis.Client
}

func NewOrder(adaptor adaptor.IAdaptor) *Order {
	return &Order{
		redis: adaptor.GetRedis(),
	}
}

func (o *Order) SetOrderCalcFee(ctx context.Context, feeUUID string, feeData string, expire time.Duration) error {
	return o.redis.Set(fmtOrderCalcFeeKey(feeUUID), feeData, expire).Err()
}

func (o *Order) GetOrderCalcFee(ctx context.Context, feeUUID string) (string, error) {
	return o.redis.Get(fmtOrderCalcFeeKey(feeUUID)).Result()
}

func (o *Order) GetAndDelOrderCalcFee(ctx context.Context, feeUUID string) (string, error) {
	key := fmtOrderCalcFeeKey(feeUUID)
	res, err := luaGetAndDelete.Run(o.redis, []string{key}).Result()
	if err != nil {
		return "", err
	}
	if res == nil || res == "" {
		return "", redis.Nil
	}
	return res.(string), nil
}

func fmtOrderLockKey(orderID int64) string {
	return fmt.Sprintf("%s:lock:order:%d", config.ServerFullName, orderID)
}
func (o *Order) GetOrderLock(ctx context.Context, orderID int64, uuid string) (bool, error) {
	redisKey := fmtOrderLockKey(orderID)
	return o.redis.SetNX(redisKey, uuid, time.Second*10).Result()
}
func (o *Order) UnLockOrder(ctx context.Context, orderID int64, uuid string) error {
	redisKey := fmtOrderLockKey(orderID)
	return luaUnlock.Run(o.redis, []string{redisKey}, uuid).Err()
}

// 订单超时取消
const (
	TimeoutOrderCancelExpire = -1 * time.Minute * 30
)

func fmtTimeoutOrderCancelZSetKey() string {
	return fmt.Sprintf("%s:order:timeout:cancel", config.ServerFullName)
}
func (o *Order) SetTimeoutOrderCancel(ctx context.Context, orderID int64) error {
	redisKey := fmtTimeoutOrderCancelZSetKey()
	_, err := o.redis.ZAdd(redisKey, redis.Z{
		Score:  float64(time.Now().UnixMilli()),
		Member: orderID,
	}).Result()
	if err != nil {
		return err
	}
	return nil
}

func (o *Order) GetTimeWaitOrderCancel(ctx context.Context) ([]int64, error) {
	redisKey := fmtTimeoutOrderCancelZSetKey()
	timeNow := time.Now()
	minTime := timeNow.AddDate(0, 0, -7).UnixMilli()
	maxTime := timeNow.Add(TimeoutOrderCancelExpire).UnixMilli()
	strList, err := o.redis.ZRangeByScore(redisKey, redis.ZRangeBy{
		Min:    gconv.String(minTime),
		Max:    gconv.String(maxTime),
		Offset: 0,
		Count:  500,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return []int64{}, nil
		}
		return nil, err
	}
	retOrderList := make([]int64, 0)
	for _, str := range strList {
		orderID := gconv.Int64(str)
		if orderID > 0 {
			retOrderList = append(retOrderList, orderID)
		}
	}
	return retOrderList, nil
}
func (o *Order) DelTimeoutOrderCancel(ctx context.Context, orderID int64) error {
	redisKey := fmtTimeoutOrderCancelZSetKey()
	_, err := o.redis.ZRem(redisKey, orderID).Result()
	return err
}

func fmtOrderPayResultHashKey() string {
	return fmt.Sprintf("%s:order:pay:result", config.ServerFullName)
}

// 订单支付结果轮询
func (o *Order) SetOrderPayResult(ctx context.Context, orderID int64) error {
	redisKey := fmtOrderPayResultHashKey()
	_, err := o.redis.HSet(redisKey, gconv.String(orderID), time.Now().UnixMilli()).Result()
	return err
}
func (o *Order) GetOrderPayResult(ctx context.Context) (map[int64]int64, error) {
	redisKey := fmtOrderPayResultHashKey()
	getMap, err := o.redis.HGetAll(redisKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return map[int64]int64{}, nil
		}
		return nil, err
	}
	retMap := make(map[int64]int64)
	for k, v := range getMap {
		retMap[gconv.Int64(k)] = gconv.Int64(v)
	}
	return retMap, nil
}
func (o *Order) DelOrderPayResult(ctx context.Context, orderID int64) error {
	redisKey := fmtOrderPayResultHashKey()
	_, err := o.redis.HDel(redisKey, gconv.String(orderID)).Result()
	return err
}

func (o *Order) CheckInPayment(ctx context.Context, orderID int64) (bool, error) {
	return o.redis.HExists(fmtOrderPayResultHashKey(), gconv.String(orderID)).Result()
}

func fmtOrderRefundResultHashKey() string {
	return fmt.Sprintf("%s:order:refund:result", config.ServerFullName)
}

func (o *Order) SetOrderRefundResult(ctx context.Context, orderID int64) error {
	redisKey := fmtOrderRefundResultHashKey()
	_, err := o.redis.HSet(redisKey, gconv.String(orderID), time.Now().UnixMilli()).Result()
	return err
}
func (o *Order) GetOrderRefundResult(ctx context.Context) (map[int64]int64, error) {
	redisKey := fmtOrderRefundResultHashKey()
	getMap, err := o.redis.HGetAll(redisKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return map[int64]int64{}, nil
		}
		return nil, err
	}
	retMap := make(map[int64]int64)
	for k, v := range getMap {
		retMap[gconv.Int64(k)] = gconv.Int64(v)
	}
	return retMap, nil
}
func (o *Order) DelOrderRefundResult(ctx context.Context, orderID int64) error {
	redisKey := fmtOrderRefundResultHashKey()
	_, err := o.redis.HDel(redisKey, gconv.String(orderID)).Result()
	return err
}

func (o *Order) renewOrderLock(ctx context.Context, orderID int64, uuid string, lockTTL time.Duration) (bool, error) {
	redisKey := fmtOrderLockKey(orderID)
	ok, err := luaRenew.Run(o.redis, []string{redisKey}, uuid, lockTTL.Seconds()).Bool()
	if ok && err == nil {
		return true, nil
	}
	return false, err
}
func (o *Order) RenewOrderLockLoop(ctx context.Context, orderID int64, uuid string, renewInterval, lockTTL time.Duration) (stop func(), err error) {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(renewInterval):
				ok, err := o.renewOrderLock(ctx, orderID, uuid, lockTTL)
				if err != nil {
					logger.Error("RenewOrderLock error", zap.Error(err), zap.Int64("order_id", orderID))
				}
				if !ok {
					logger.Warn("RenewOrderLock lost, lock taken by others", zap.Int64("order_id", orderID))
					return
				}
			}
		}
	}()
	return cancel, nil
}
