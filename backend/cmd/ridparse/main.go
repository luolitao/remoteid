// ridparse — Remote ID Monitor 数据解析命令行工具
//
// 支持三种模式：
//   1. 实时抓包:   ridparse -iface wlan1
//   2. 离线分析:   ridparse -file capture.pcap
//   3. TUI 监控:   ridparse -tui [-url http://rpi5.lan:8000]
//
// TUI 模式通过 HTTP API + WebSocket 连接后端服务，在终端内实时展示无人机列表，
// 支持自动刷新、连接状态指示、在线时长统计。
package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"remoteid-monitor/internal/drone"
	"remoteid-monitor/pkg/types"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/pcapgo"
	"github.com/gorilla/websocket"
)

// ANSI 颜色 / 控制
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorGray   = "\033[90m"

	clearScreen = "\033[H\033[2J"
	hideCursor  = "\033[?25l"
	showCursor  = "\033[?25h"

	version = "1.0.0"
)

var (
	stats struct {
		totalPackets int
		dronePackets int
		uniqueDrones map[string]bool
		startTime    time.Time
	}
	parser = drone.NewParser()
)

func main() {
	iface := flag.String("iface", "", "监控网络接口（实时抓包模式）")
	file := flag.String("file", "", "pcap/pcapng 文件路径（离线分析模式）")
	tui := flag.Bool("tui", false, "TUI 监控模式（连接后端 API + WebSocket）")
	apiURL := flag.String("url", "http://127.0.0.1:8000", "后端 API 地址（TUI 模式使用）")
	refresh := flag.Duration("refresh", 2*time.Second, "TUI 刷新间隔")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `ridparse — Remote ID Monitor (ASTM F3411 / GB42590) 数据解析工具

用法:
  ridparse -iface wlan1                     实时抓包并解析
  ridparse -file capture.pcap               离线分析 pcap 文件
  ridparse -tui                             TUI 监控模式（连接本地后端）
  ridparse -tui -url http://rpi5.lan:8000   TUI 连接指定后端

选项:
`)
		flag.PrintDefaults()
	}
	flag.Parse()

	stats.uniqueDrones = make(map[string]bool)
	stats.startTime = time.Now()

	switch {
	case *tui:
		runTUI(*apiURL, *refresh)
	case *file != "":
		parsePcapFile(*file)
	case *iface != "":
		parseLive(*iface)
	default:
		flag.Usage()
		os.Exit(1)
	}
}

// ============================================================
// 模式 1: TUI 监控（API + WebSocket）
// ============================================================

type droneState struct {
	mu       sync.RWMutex
	drones   map[string]*types.DroneData
	lastSeen map[string]time.Time
}

func (ds *droneState) update(d *types.DroneData) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.drones[d.MAC] = d
	ds.lastSeen[d.MAC] = time.Now()
}

func (ds *droneState) removeStale(maxAge time.Duration) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	now := time.Now()
	for mac, t := range ds.lastSeen {
		if now.Sub(t) > maxAge {
			delete(ds.drones, mac)
			delete(ds.lastSeen, mac)
		}
	}
}

func (ds *droneState) sortedList() []*types.DroneData {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	list := make([]*types.DroneData, 0, len(ds.drones))
	for _, d := range ds.drones {
		list = append(list, d)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].LastSeen.After(list[j].LastSeen)
	})
	return list
}

func runTUI(apiURL string, refreshInterval time.Duration) {
	fmt.Print(hideCursor)
	defer fmt.Print(showCursor)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 捕获 Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	state := &droneState{
		drones:   make(map[string]*types.DroneData),
		lastSeen: make(map[string]time.Time),
	}

	wsURL := wsURLFromHTTP(apiURL) + "/ws"
	wsConnected := false
	var wsMu sync.Mutex

	// WebSocket 连接 goroutine
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				wsMu.Lock()
				wsConnected = false
				wsMu.Unlock()
				time.Sleep(3 * time.Second)
				continue
			}

			wsMu.Lock()
			wsConnected = true
			wsMu.Unlock()

			for {
				_, msg, err := conn.ReadMessage()
				if err != nil {
					break
				}
				var wsMsg types.WSMessage
				if err := json.Unmarshal(msg, &wsMsg); err != nil {
					continue
				}
				if wsMsg.Type == "drone_update" {
					data, _ := json.Marshal(wsMsg.Data)
					var d types.DroneData
					if json.Unmarshal(data, &d) == nil {
						state.update(&d)
					}
				}
			}

			conn.Close()
			wsMu.Lock()
			wsConnected = false
			wsMu.Unlock()
			time.Sleep(3 * time.Second)
		}
	}()

	// 初始加载（通过 HTTP）
	go func() {
		loadFromAPI(apiURL, state)
	}()

	// TUI 渲染循环
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	lastFetch := time.Now()
	fetchInterval := 5 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 定期全量拉取兜底
			if time.Since(lastFetch) > fetchInterval {
				loadFromAPI(apiURL, state)
				lastFetch = time.Now()
			}

			state.removeStale(30 * time.Second)

			wsMu.Lock()
			connected := wsConnected
			wsMu.Unlock()

			renderTUI(state.sortedList(), connected, apiURL)
		}
	}
}

func wsURLFromHTTP(httpURL string) string {
	u, err := url.Parse(httpURL)
	if err != nil {
		return "ws://127.0.0.1:8000"
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	return u.String()
}

func loadFromAPI(apiURL string, state *droneState) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(apiURL + "/api/drones")
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var result struct {
		Drones []*types.DroneData `json:"drones"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return
	}

	for _, d := range result.Drones {
		state.update(d)
	}
}

func renderTUI(drones []*types.DroneData, wsConnected bool, apiURL string) {
	fmt.Print(clearScreen)

	elapsed := time.Since(stats.startTime)
	wsStatus := colorRed + "● 断线" + colorReset
	if wsConnected {
		wsStatus = colorGreen + "● 在线" + colorReset
	}

	// 顶部状态栏
	sep := colorCyan + strings.Repeat("═", 62) + colorReset
	div := colorCyan + strings.Repeat("─", 62) + colorReset

	fmt.Println(sep)
	fmt.Println(colorCyan + "  📡 Remote ID Monitor v" + version + "  |  后端: " + apiURL + "  |  WS: " + wsStatus + "  |  运行: " + elapsed.Round(time.Second).String() + colorReset)
	fmt.Println(div)

	if len(drones) == 0 {
		fmt.Println("\n  " + colorGray + "  暂无无人机数据，等待接收..." + colorReset + "\n")
	} else {
		for _, d := range drones {
			stdColor := colorBlue
			stdLabel := d.Standard
			if strings.HasPrefix(d.Standard, "GB42590") {
				stdColor = colorGreen
			}

			loc := "-"
			if d.Latitude != 0 || d.Longitude != 0 {
				loc = fmt.Sprintf("%.4f, %.4f", d.Latitude, d.Longitude)
			}

			alt := "-"
			if d.Altitude != 0 {
				alt = fmt.Sprintf("%.1f m", d.Altitude)
			}

			spd := "-"
			if d.Speed != 0 {
				spd = fmt.Sprintf("%.1f m/s", d.Speed)
			}

			sig := d.SignalStrength
			if sig == "" {
				sig = "-"
			}

			uaID := d.UASID
			if uaID == "" {
				uaID = "-"
			}

			sinceLast := time.Since(d.LastSeen)
			ageTag := ""
			if sinceLast > 10*time.Second {
				ageTag = " " + colorYellow + "●" + colorReset
			}

			// 卡片标题行
			title := fmt.Sprintf("%s%s%s  %s%s%s  %s%s%s",
				colorYellow, d.MAC, colorReset,
				stdColor, stdLabel, colorReset,
				colorGray, sinceLast.Round(time.Second).String(), colorReset) + ageTag
			fmt.Println("  " + colorCyan + "┌─ " + title + colorReset)

			// UA_ID 行
			fmt.Println("  " + colorCyan + "│" + colorReset + "  UA_ID: " + colorBold + uaID + colorReset)

			// 位置行
			fmt.Println("  " + colorCyan + "│" + colorReset + "  位置: " + loc + "  高度: " + alt + "  速度: " + spd)

			// 信号行
			fmt.Println("  " + colorCyan + "│" + colorReset + "  信号: " + sig)

			// 卡片底部
			fmt.Println("  " + colorCyan + "└" + strings.Repeat("─", 60) + "┘" + colorReset)
			fmt.Println()
		}
	}

	// 底部状态栏
	fmt.Println(div)
	fmt.Println(colorCyan + "  无人机: " + colorBold + fmt.Sprintf("%d", len(drones)) + colorReset + "  |  Ctrl+C 退出  |  刷新: 2s (API: 5s)" + colorReset)
	fmt.Println(sep)
}

// ============================================================
// 模式 2: 离线分析 pcap 文件
// ============================================================

func parsePcapFile(filename string) {
	f, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法打开文件: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	var packetSource *gopacket.PacketSource
	r, err := pcapgo.NewNgReader(f, pcapgo.DefaultNgReaderOptions)
	if err == nil {
		packetSource = gopacket.NewPacketSource(r, layers.LinkTypeEthernet)
	} else {
		f.Seek(0, 0)
		r2, err2 := pcapgo.NewReader(f)
		if err2 != nil {
			fmt.Fprintf(os.Stderr, "无法解析 pcap 文件: %v (pcapng) / %v (pcap)\n", err, err2)
			os.Exit(1)
		}
		packetSource = gopacket.NewPacketSource(r2, r2.LinkType())
	}

	fmt.Printf("%s📡 离线分析: %s%s\n\n", colorGreen, filename, colorReset)
	printHeader()

	for packet := range packetSource.Packets() {
		if packet == nil {
			continue
		}
		stats.totalPackets++
		processPacket(packet)
	}

	printStats()
}

// ============================================================
// 模式 3: 实时抓包
// ============================================================

func parseLive(iface string) {
	handle, err := pcap.OpenLive(iface, 1024, true, pcap.BlockForever)
	if err != nil {
		if strings.Contains(err.Error(), "Permission denied") {
			fmt.Fprintf(os.Stderr, "权限不足。请运行: sudo setcap 'cap_net_raw,cap_net_admin+eip' ridparse\n")
		} else {
			fmt.Fprintf(os.Stderr, "打开网卡失败: %v\n", err)
		}
		os.Exit(1)
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packetSource.NoCopy = true

	fmt.Printf("%s📡 实时抓包: %s (Ctrl+C 退出)%s\n\n", colorGreen, iface, colorReset)
	printHeader()

	for packet := range packetSource.Packets() {
		if packet == nil {
			continue
		}
		stats.totalPackets++
		processPacket(packet)
	}
}

// ============================================================
// 共享：解析 + 输出
// ============================================================

func printHeader() {
	fmt.Printf("%s%-20s %-10s %-10s %-22s %-12s %-10s %-8s %-8s%s\n",
		colorBold,
		"MAC", "标准", "帧类型", "UA_ID", "纬度", "经度", "高度(m)", "速度(m/s)",
		colorReset)
	fmt.Printf("%s%s%s\n", colorCyan, strings.Repeat("─", 110), colorReset)
}

func processPacket(packet gopacket.Packet) {
	signalStrength := -100
	if rl := packet.Layer(layers.LayerTypeRadioTap); rl != nil {
		if rt, ok := rl.(*layers.RadioTap); ok {
			signalStrength = int(rt.DBMAntennaSignal)
		}
	}

	dl := packet.Layer(layers.LayerTypeDot11)
	if dl == nil {
		return
	}
	dot11, ok := dl.(*layers.Dot11)
	if !ok {
		return
	}

	srcMAC := ""
	if dot11.Address2 != nil {
		srcMAC = dot11.Address2.String()
	}
	if srcMAC == "" {
		return
	}

	frameSubtype := getMgmtSubtype(dot11.Type)
	if frameSubtype == 0 && dot11.Type != layers.Dot11TypeMgmtAssociationReq {
		return
	}

	if signalStrength < -95 {
		return
	}

	raw := packet.Data()

	var messages []types.DroneMessage
	var standard string

	if frameSubtype == 13 {
		if !isValidNANRemoteID(raw) {
			return
		}
		msgs, err := parser.ParseNANFrame(raw)
		if err != nil || len(msgs) == 0 {
			return
		}
		messages = msgs
		standard = "ASTM(NAN)"
	} else {
		if !isValidRemoteID(raw) {
			return
		}
		msgs, err := parser.ParseFrame(raw)
		if err != nil || len(msgs) == 0 {
			return
		}
		messages = msgs
		if len(msgs) > 0 {
			standard = msgs[0].Standard
		}
	}

	stats.dronePackets++
	stats.uniqueDrones[srcMAC] = true

	printDroneInfo(srcMAC, standard, messages, signalStrength)
}

func getMgmtSubtype(t layers.Dot11Type) uint8 {
	switch t {
	case layers.Dot11TypeMgmtBeacon:
		return 8
	case layers.Dot11TypeMgmtProbeReq:
		return 4
	case layers.Dot11TypeMgmtProbeResp:
		return 5
	case layers.Dot11TypeMgmtAuthentication:
		return 11
	case layers.Dot11TypeMgmtAssociationReq:
		return 0
	case layers.Dot11TypeMgmtAssociationResp:
		return 1
	case layers.Dot11TypeMgmtDeauthentication:
		return 12
	case layers.Dot11TypeMgmtAction:
		return 13
	default:
		return 0
	}
}

func isValidRemoteID(raw []byte) bool {
	for i := 0; i < len(raw)-4; i++ {
		// 检查 ASD-STAN OUI (FA:0B:BC) + OUI_Type (0x0D)
		if raw[i] == 0xFA && raw[i+1] == 0x0B && raw[i+2] == 0xBC {
			if i+4 < len(raw) && raw[i+3] == 0x0D {
				payload := raw[i+4:]
				if len(payload) < 1 {
					continue
				}

				// 优先检查 payload[1]（跳过可能的 Message Counter）是否为 GB42590
				if len(payload) > 1 {
					nextNibble := (payload[1] >> 4) & 0x0F
					nextLow := payload[1] & 0x0F
					// GB42590 Packed 格式
					if nextNibble == 0xF {
						return true
					}
					// GB42590 单消息
					if nextLow == 0x1 && nextNibble <= 5 {
						return true
					}
				}

				firstNibble := (payload[0] >> 4) & 0x0F
				lowNibble := payload[0] & 0x0F

				// ASTM: 高4位=协议版本(2), 低4位=消息类型(0-5)
				if firstNibble == 2 && lowNibble <= 5 {
					return true
				}

				// GB42590 单消息（无 Message Counter）
				if lowNibble == 0x1 && firstNibble <= 5 {
					return true
				}

				// 旧版格式
				if firstNibble <= 5 {
					return true
				}
			}
		}

		// 检查旧版 ASTM OUI (06:05:04) + OUI_Type (0xFD)
		if raw[i] == 0x06 && raw[i+1] == 0x05 && raw[i+2] == 0x04 {
			if i+4 < len(raw) && raw[i+3] == 0xFD {
				payload := raw[i+4:]
				if len(payload) < 1 {
					continue
				}
				firstNibble := (payload[0] >> 4) & 0x0F
				lowNibble := payload[0] & 0x0F
				if firstNibble == 2 && lowNibble <= 5 {
					return true
				}
			}
		}
	}
	return false
}

func isValidNANRemoteID(raw []byte) bool {
	hasWiFiAlliance := false
	hasRemoteID := false
	for i := 0; i < len(raw)-3; i++ {
		if raw[i] == 0x50 && raw[i+1] == 0x6F && raw[i+2] == 0x9A {
			if i+3 < len(raw) && raw[i+3] == 0x13 {
				hasWiFiAlliance = true
			}
		}
		if raw[i] == 0xFA && raw[i+1] == 0x0B && raw[i+2] == 0xBC {
			if i+3 < len(raw) && raw[i+3] == 0x0D {
				hasRemoteID = true
			}
		}
	}
	return hasWiFiAlliance && hasRemoteID
}

func printDroneInfo(mac, standard string, messages []types.DroneMessage, signal int) {
	var (
		uasID    string
		uaType   string
		lat      string
		lon      string
		alt      string
		speed    string
		msgTypes []string
	)

	for _, msg := range messages {
		msgTypes = append(msgTypes, msg.MessageType)
		switch msg.MessageType {
		case "basic_id":
			if v, ok := msg.Data["uas_id"]; ok && v != "" {
				uasID = v
			}
			if v, ok := msg.Data["ua_type"]; ok && v != "" {
				uaType = v
			}
		case "location":
			if v, ok := msg.Data["latitude"]; ok {
				lat = v
			}
			if v, ok := msg.Data["longitude"]; ok {
				lon = v
			}
			if v, ok := msg.Data["altitude_baro"]; ok {
				alt = v
			} else if v, ok := msg.Data["altitude_geo"]; ok {
				alt = v
			}
			if v, ok := msg.Data["speed_h"]; ok {
				speed = v
			}
		}
	}

	stdColor := colorBlue
	if strings.Contains(standard, "GB42590") {
		stdColor = colorGreen
	}

	uasID = truncate(uasID, 20)
	uaType = truncate(uaType, 10)
	if lat == "" {
		lat = "-"
	}
	if lon == "" {
		lon = "-"
	}
	if alt == "" {
		alt = "-"
	}
	if speed == "" {
		speed = "-"
	}

	fmt.Printf("%s%-20s%s %s%-10s%s %-10s %-22s %-12s %-10s %-8s %-8s %s%d dBm%s\n",
		colorYellow, mac, colorReset,
		stdColor, standard, colorReset,
		strings.Join(msgTypes, "+"),
		uaType+" | "+uasID,
		lat, lon, alt, speed,
		colorWhite, signal, colorReset,
	)
}

func printStats() {
	elapsed := time.Since(stats.startTime)
	fmt.Printf("\n%s══════════════════════════════════════%s\n", colorCyan, colorReset)
	fmt.Printf("%s  总包数: %d  无人机包: %d  唯一无人机: %d  耗时: %v%s\n",
		colorBold, stats.totalPackets, stats.dronePackets, len(stats.uniqueDrones), elapsed.Round(time.Millisecond), colorReset)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

var _ = binary.LittleEndian
