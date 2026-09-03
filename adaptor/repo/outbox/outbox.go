package outbox

import (
	"context"
	"gorm.io/gorm"
	"mall/adaptor"
	"mall/adaptor/repo/model"
	"time"
)

type IOutbox interface {
	// CreateOutbox 在调用方提供的事务连接里插入一条待发布事件（status=0）。
	CreateOutbox(ctx context.Context, db *gorm.DB, eventType, payload string) error
	// GetPendingOutbox 查询待发布事件， 按id升序取limit条。
	GetPendingOutbox(ctx context.Context, limit int) ([]*model.OrderEventOutbox, error)
	// MarkPublished 把事件标记为已发布。
	MarkPublished(ctx context.Context, id int64) error
}

type Outbox struct {
	db *gorm.DB
}

func NewOutbox(adaptor adaptor.IAdaptor) *Outbox {
	return &Outbox{db: adaptor.GetDB()}
}

func (o *Outbox) CreateOutbox(ctx context.Context, db *gorm.DB, eventType, payload string) error {
	return db.WithContext(ctx).Create(&model.OrderEventOutbox{
		EventType:    eventType,
		EventPayload: payload,
		Status:       0,
		RetryCount:   0,
		CreateAt:     time.Now().UnixMilli(),
		PublishAt:    nil,
	}).Error
}

func (o *Outbox) GetPendingOutbox(ctx context.Context, limit int) ([]*model.OrderEventOutbox, error) {
	list := make([]*model.OrderEventOutbox, 0)
	err := o.db.WithContext(ctx).Where("status = ?", 0).Order("id asc").Limit(limit).Find(&list).Error
	return list, err
}

func (o *Outbox) MarkPublished(ctx context.Context, id int64) error {
	return o.db.WithContext(ctx).Model(&model.OrderEventOutbox{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     1,
		"publish_at": time.Now().UnixMilli(),
	}).Error
}
