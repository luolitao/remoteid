package capture

import (
	"context"
	"remoteid-monitor/pkg/types"
	"time"
)

// RawPacket 统一的数据包结构，屏蔽底层传输介质差异
type RawPacket struct {
	Source    string    // 来源标识，如 "wifi:54:75:95:xx:xx:xx"
	Timestamp time.Time // 捕获时间
	SignalDBM int       // 信号强度
	Payload   []byte    // 原始载荷 (Parser 将直接解析此字段)
	Transport string    // 传输类型: "wifi", "ble", "cellular"
}

// Capturer 捕获器接口，所有传输介质必须实现此接口
type Capturer interface {
	Name() string
	Start(ctx context.Context, out chan<- RawPacket) error
	Stop() error
	GetStats() types.CaptureStats // 新增：获取该捕获器的实时统计
}
