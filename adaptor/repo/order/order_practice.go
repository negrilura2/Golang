package order

import (
	"context"
	"gorm.io/gorm"
	"mall/adaptor/repo/model"
	"mall/adaptor/repo/query"
	"mall/consts"
)

func (o *Order) MarkPayedGorm(ctx context.Context, orderID, paymentAt int64) (bool, error) {
	res := o.db.WithContext(ctx).Model(&model.Order{}).Where("id = ? AND status = ?", orderID, consts.OrderStatusWaitPay).Updates(
		map[string]interface{}{ //表绑定 必须写model, 否则“Table not set"
			//条件：字符串+？
			"status":     consts.OrderStatusPayed, //列名： 字符串， 拼错运行才炸
			"payment_at": paymentAt,
		}) //返回值: *.gorm.DB 一个值， 字段直接.Error/.RowsAffected
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (o *Order) MarkPayedGen(ctx context.Context, db *gorm.DB, orderID, paymentAt int64) (bool, error) {
	qs := query.Use(db).Order //表绑定：天生就是orders表， 不用写Model
	updateMap := map[string]interface{}{
		qs.Status.ColumnName().String():    consts.OrderStatusPayed, //列名： Go符号 qs.Status， 拼错编译报错
		qs.PaymentAt.ColumnName().String(): paymentAt,
	}
	info, err := qs.WithContext(ctx).Where(qs.ID.Eq(orderID), qs.Status.Eq(consts.OrderStatusWaitPay)).Updates(updateMap) //条件： 类型安全方法
	//返回值： *(ResultInfo, error)两个值
	if err != nil {
		return false, err
	}
	return info.RowsAffected > 0, nil
}
