package snowflake

import (
	"errors"
	"sync"
	"time"
)

const (
	epoch        int64 = 1288834974657 // Twitter 默认纪元
	workerBits   uint8 = 10
	sequenceBits uint8 = 12

	WorkerIDMax  = -1 ^ (-1 << workerBits)   // 1023
	sequenceMask = -1 ^ (-1 << sequenceBits) // 4095
)

type Node struct {
	mu        sync.Mutex
	workerID  int64
	lastStamp int64
	sequence  int64
}

// NewNode 传入 workerID（0~1023）即可
func NewNode(workerID int64) (*Node, error) {
	if workerID < 0 || workerID > WorkerIDMax {
		return nil, errors.New("worker id out of range")
	}
	return &Node{workerID: workerID}, nil
}

// GetNextID 生成下一个 ID
func (n *Node) GetNextID() int64 {
	n.mu.Lock()
	defer n.mu.Unlock()

	now := time.Now().UnixMilli()
	if now < n.lastStamp {
		time.Sleep(time.Duration(n.lastStamp-now) * time.Millisecond)
		now = time.Now().UnixMilli()
	}

	if now == n.lastStamp {
		n.sequence = (n.sequence + 1) & sequenceMask
		if n.sequence == 0 {
			for now <= n.lastStamp {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		n.sequence = 0
	}

	n.lastStamp = now
	return (now-epoch)<<(workerBits+sequenceBits) | n.workerID<<sequenceBits | n.sequence
}
