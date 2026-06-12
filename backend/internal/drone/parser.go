package drone

import (
	"encoding/hex" // 引入 hex 包用于打印可视化的十六进制原始流
	"errors"
	"log" // 引入标准日志包
	"sync"
)

var (
	ErrUnsupportedProtocol = errors.New("unsupported remote id protocol")
	ErrPacketTooShort      = errors.New("packet length is too short for decoding")
)

// DebugMode 调试开关，设为 true 时将输出完整的包追踪链路
var DebugMode = false

// Parser 定义了所有 Remote ID 协议解析器的通用行为契约
type Parser interface {
	Match(payload []byte) bool
	Parse(payload []byte) (*UnpackedTelemetry, error)
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
	if DebugMode {
		log.Printf("[DEBUG-INIT] 成功注册 Remote ID 解析器: %s", p.Name())
	}
}

// RouteAndParse 动态路由引擎：带高级调试跟踪功能
func (r *ParserRegistry) RouteAndParse(payload []byte) (*UnpackedTelemetry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. 跟踪流入的原始数据包
	if DebugMode {
		log.Printf("[DEBUG-TRACE] 📥 收到待路由数据包 | 长度: %d 字节 | 原始Hex: %s", len(payload), hex.EncodeToString(payload))
	}

	// 2. 遍历解析器链路
	for idx, parser := range r.parsers {
		if DebugMode {
			log.Printf("[DEBUG-TRACE]  ├── [%d] 正在尝试匹配解析器: %s ...", idx, parser.Name())
		}

		// 检查是否匹配特征（如 OUI）
		if parser.Match(payload) {
			if DebugMode {
				log.Printf("[DEBUG-TRACE]  ├── 🎯 [命中] 数据包特征符合 %s 协议规范，进入深度解析", parser.Name())
			}

			// 执行具体协议的 Parse 解包
			telemetry, err := parser.Parse(payload)
			if err != nil {
				if DebugMode {
					log.Printf("[DEBUG-TRACE]  └── ❌ [%s] 解析失败! 错误原因: %v", parser.Name(), err)
				}
				return nil, err
			}

			// 解析成功跟踪
			if DebugMode {
				log.Printf("[DEBUG-TRACE]  └── ✅ [%s] 解析成功! 路由流结束。", parser.Name())
			}
			return telemetry, nil
		}
	}

	// 3. 没有任何协议解析器认领此包
	if DebugMode {
		log.Printf("[DEBUG-TRACE] └── ⚠️ 遍历结束：未找到任何匹配该数据包特征的 Remote ID 解析器。")
	}
	return nil, ErrUnsupportedProtocol
}
