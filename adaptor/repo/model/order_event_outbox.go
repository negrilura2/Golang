package model

const TableNameOrderEventOutbox = "order_event_outbox"

// OrderEventOutbox 事务性发件箱： 业务事务内落一条事件， 事务外异步发布
type OrderEventOutbox struct {
	ID           int64  `gorm:"column:id;primaryKey;autoIncrement"`
	EventType    string `gorm:"column:event_type"`
	EventPayload string `gorm:"column:event_payload"`
	Status       int32  `gorm:"column:status"` // 0待发布 1已发布 2失败
	RetryCount   int32  `gorm:"column:retry_count"`
	CreateAt     int64  `gorm:"column:create_at"`
	PublishAt    *int64 `gorm:"column:publish_at"`
}

func (*OrderEventOutbox) TableName() string { return TableNameOrderEventOutbox }
