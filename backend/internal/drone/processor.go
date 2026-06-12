package drone

import (
	"errors"
	"log"
	"sync"
	"time"
)

type Processor struct {
	mu          sync.RWMutex
	drones      map[string]*TrackedDrone
	broadcastCh chan *TrackedDrone
	stopCh      chan struct{}
}

func NewProcessor(broadcastCh chan *TrackedDrone) *Processor {
	p := &Processor{
		drones:      make(map[string]*TrackedDrone),
		broadcastCh: broadcastCh,
		stopCh:      make(chan struct{}),
	}
	// 5秒扫描一次，清理30秒未更新的幽灵节点
	go p.startStateCleaner(5*time.Second, 30*time.Second)
	return p
}

func (p *Processor) ProcessPacket(payload []byte, mac string, rssi int) {
	if len(payload) == 0 {
		return
	}

	// 动态分配路由解包
	telemetry, err := DefaultRegistry.RouteAndParse(payload)
	if err != nil {
		if errors.Is(err, ErrUnsupportedProtocol) {
			return // 忽略普通Wi-Fi背景噪音
		}
		log.Printf("[Processor] 解析异常: %v", err)
		return
	}

	dKey := telemetry.UASID
	if dKey == "" {
		if mac != "" {
			dKey = "MAC_" + mac
		} else {
			return
		}
	}

	p.mu.Lock()
	drone, exists := p.drones[dKey]
	if !exists {
		drone = &TrackedDrone{}
		p.drones[dKey] = drone
	}
	drone.Telemetry = telemetry
	drone.LastSeen = time.Now()
	drone.MACAddress = mac
	drone.RSSI = rssi
	p.mu.Unlock()

	// 非阻塞发送广播
	select {
	case p.broadcastCh <- drone:
	default:
	}
}

func (p *Processor) GetActiveDrones() []*TrackedDrone {
	p.mu.RLock()
	defer p.mu.RUnlock()

	list := make([]*TrackedDrone, 0, len(p.drones))
	for _, drone := range p.drones {
		list = append(list, drone)
	}
	return list
}

func (p *Processor) Close() {
	close(p.stopCh)
}

func (p *Processor) startStateCleaner(interval, timeout time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.mu.Lock()
			now := time.Now()
			for id, drone := range p.drones {
				if now.Sub(drone.LastSeen) > timeout {
					log.Printf("[Processor] 目标丢失，移除状态看板: %s", id)
					delete(p.drones, id)
				}
			}
			p.mu.Unlock()
		case <-p.stopCh:
			return
		}
	}
}
