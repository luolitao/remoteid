package drone

import (
	"errors"
	"sync"
)

var (
	ErrUnsupportedProtocol = errors.New("unsupported remote id protocol")
	ErrPacketTooShort      = errors.New("packet length is too short for decoding")
)

// Parser 定义了所有 Remote ID 协议解析器的通用行为契约
type Parser interface {
	// Match 根据报文特征快速判断是否由本解析器处理
	Match(payload []byte) bool

	// Parse 将原始空口字节解析为系统统一的标准化输出结构体
	Parse(payload []byte) (*UnpackedTelemetry, error)

	// Name 返回该协议的规范名称
	Name() string
}

// ParserRegistry 提供动态路由与解析器管理中心
type ParserRegistry struct {
	mu      sync.RWMutex
	parsers []Parser
}

// DefaultRegistry 全局默认注册单例
var DefaultRegistry = &ParserRegistry{}

// RegisterParser 允许各协议文件在 init() 时自动上报注册
func (r *ParserRegistry) RegisterParser(p Parser) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.parsers = append(r.parsers, p)
}

// RouteAndParse 动态路由引擎：遍历所有解析器，谁匹配谁执行
func (r *ParserRegistry) RouteAndParse(payload []byte) (*UnpackedTelemetry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, parser := range r.parsers {
		if parser.Match(payload) {
			return parser.Parse(payload)
		}
	}
	return nil, ErrUnsupportedProtocol
}
