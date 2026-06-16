// internal/debug/debug.go
package debug

import (
	"sync"
	"sync/atomic"
	"time"
)

// 全局统计指标（atomic 保证零锁高性能）
var (
	TotalCaptured  atomic.Int64
	BeaconFiltered atomic.Int64
	OUIFiltered    atomic.Int64
	ParseSuccess   atomic.Int64
	ParseFailed    atomic.Int64
)

// PacketRecord 单条抓包调试记录
type PacketRecord struct {
	Time     time.Time `json:"time"`
	MAC      string    `json:"mac"`
	RSSI     int       `json:"rssi"`
	Payload  string    `json:"payload_hex"`
	Stage    string    `json:"stage"` // sniffed | oui_matched | parsed_ok | parsed_err
	Protocol string    `json:"protocol,omitempty"`
	Error    string    `json:"error,omitempty"`
}

// Debugger 有界环形缓存（默认保留最近 200 条，防止 OOM）
var Debugger = newDebugger(200)

type debugger struct {
	mu      sync.Mutex
	records []PacketRecord
	cap     int
}

func newDebugger(cap int) *debugger {
	return &debugger{cap: cap, records: make([]PacketRecord, 0, cap)}
}

// Add 安全写入（自动淘汰最旧数据）
func (d *debugger) Add(r PacketRecord) {
	d.mu.Lock()
	defer d.mu.Unlock()
	r.Time = time.Now()
	if len(d.records) >= d.cap {
		d.records = d.records[1:]
	}
	d.records = append(d.records, r)
}

// Get 按数量获取最新记录
func (d *debugger) Get(limit int) []PacketRecord {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(d.records)
	if limit <= 0 || limit > n {
		limit = n
	}
	out := make([]PacketRecord, limit)
	copy(out, d.records[n-limit:])
	return out
}

// ResetStats 清空计数器
func ResetStats() {
	TotalCaptured.Store(0)
	BeaconFiltered.Store(0)
	OUIFiltered.Store(0)
	ParseSuccess.Store(0)
	ParseFailed.Store(0)
}
