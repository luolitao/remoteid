<template>
  <div id="sidebar_container" :style="{ width: width + 'px' }">
    <!-- 侧边栏头部 -->
    <div id="sidebar_header">
      <div class="flex items-center gap-2">
        <span class="status-dot" :class="dataStaleWarning ? 'error' : 'live'"></span>
        <span id="header_title">Remote ID</span>
      </div>
      <div class="flex items-center gap-1">
        <span id="header_count">{{ store.activeDrones.length }} drones</span>
        <button
          class="text-white opacity-60 hover:opacity-100"
          title="Hide sidebar"
          style="font-size: 14px; line-height: 1"
          @click="$emit('close')"
        >
          ✕
        </button>
      </div>
    </div>

    <!-- 数据过期警告条 -->
    <div v-if="dataStaleWarning" class="stale-warning-bar">
      ⚠ 数据已超过 60 秒未更新 — 请检查后端抓包服务是否正常运行
    </div>

    <!-- 搜索栏 -->
    <div id="search_bar">
      <input
        id="search_input"
        v-model="searchQuery"
        type="text"
        placeholder="Filter by ID/MAC/Type..."
      />
    </div>

    <!-- 无人机表格 -->
    <div id="sidebar_canvas">
      <table id="planesTable">
        <thead>
          <tr class="aircraft_table_header">
            <th class="icaoCodeColumn">ID</th>
            <th
              v-for="col in visibleColumnList"
              :key="col.key"
              :class="['text-right', col.align === 'right' ? 'font-mono' : '']"
            >
              {{ col.label }}
            </th>
            <th class="text-right font-mono">Seen</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="drone in filteredDrones"
            :key="drone.mac"
            class="plane_table_row"
            :class="getRowClass(drone)"
            :style="
              selectedDrone?.mac === drone.mac
                ? 'background-color: var(--ACCENT, #3182ce); color: #fff;'
                : ''
            "
            @click="selectDrone(drone)"
          >
            <td class="icaoCodeColumn">
              <div class="font-bold">{{ drone.uas_id || shortenMac(drone.mac, false) }}</div>
              <div class="opacity-50" style="font-size: 9px">{{ drone.mac }}</div>
            </td>
            <td
              v-for="col in visibleColumnList"
              :key="col.key"
              :class="col.align === 'right' ? 'text-right font-mono' : ''"
              :style="{ fontSize: col.size || 'var(--FS2, 12px)' }"
            >
              <template v-if="col.key === 'ua_type'">{{ drone.ua_type || '?' }}</template>
              <template v-else-if="col.key === 'standard'">{{ drone.standard || '-' }}</template>
              <template v-else-if="col.key === 'source'">{{ drone.source || '-' }}</template>
              <template v-else-if="col.key === 'altitude'">{{
                drone.altitude != null ? Math.round(drone.altitude) + 'm' : '-'
              }}</template>
              <template v-else-if="col.key === 'latitude'">{{
                drone.latitude != null ? drone.latitude.toFixed(4) : '-'
              }}</template>
              <template v-else-if="col.key === 'longitude'">{{
                drone.longitude != null ? drone.longitude.toFixed(4) : '-'
              }}</template>
              <template v-else-if="col.key === 'speed'">{{
                drone.speed != null ? drone.speed.toFixed(1) + 'm/s' : '-'
              }}</template>
              <template v-else-if="col.key === 'heading'">{{
                drone.heading != null ? drone.heading + '°' : '-'
              }}</template>
              <template v-else-if="col.key === 'status'">{{ drone.status || '-' }}</template>
              <template v-else-if="col.key === 'id_type'">{{ drone.id_type || '-' }}</template>
              <template v-else-if="col.key === 'signal'">{{
                drone.signal_strength || '-'
              }}</template>
              <template v-else-if="col.key === 'first_seen'">{{
                formatTimeShort(drone.first_seen)
              }}</template>
              <template v-else-if="col.key === 'operator_lat'">{{
                drone.operator_latitude != null ? drone.operator_latitude.toFixed(4) : '-'
              }}</template>
              <template v-else-if="col.key === 'operator_lng'">{{
                drone.operator_longitude != null ? drone.operator_longitude.toFixed(4) : '-'
              }}</template>
              <template v-else-if="col.key === 'area_radius'">{{
                drone.area_radius_m != null ? drone.area_radius_m + 'm' : '-'
              }}</template>
              <template v-else-if="col.key === 'region'">{{
                drone.classification_region || '-'
              }}</template>
              <template v-else>-</template>
            </td>
            <td class="text-right font-mono" style="font-size: var(--FS2, 12px)">
              {{ timeAgoShort(drone.last_seen) }}
            </td>
          </tr>
          <tr v-if="filteredDrones.length === 0">
            <td
              :colspan="2 + visibleColumnList.length"
              class="py-4 text-center"
              style="color: var(--BGCOLOR2, #999)"
            >
              No drones detected
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 选中无人机详情（侧边栏内，表格下方） -->
    <div v-if="showInfoBlock && selectedDrone" id="sidebar_drone_detail">
      <!-- 顶部 Header -->
      <div
        class="flex items-center justify-between px-2 py-1 text-xs font-bold text-white"
        style="background-color: var(--ACCENT)"
      >
        <span>{{ selectedDrone.uas_id || 'Unknown UAS' }}</span>
        <button class="text-xs text-white" @click="closeInfoBlock">✕</button>
      </div>

      <!-- MAC 地址 -->
      <div class="infoBlockSection" style="padding-bottom: 2px">
        <div
          class="font-mono uppercase"
          style="font-size: 10px; color: var(--TXTCOLOR2); opacity: 0.7"
        >
          {{ selectedDrone.mac }}
        </div>
      </div>

      <!-- 核心坐标区 (保持醒目) -->
      <div
        class="infoBlockSection"
        style="
          border-top: 1px solid var(--BGCOLOR2);
          padding-top: 4px;
          padding-bottom: 4px;
          background: rgba(0, 0, 0, 0.02);
        "
      >
        <div class="infoRow">
          <span class="infoHeading">Lat:</span
          ><span class="infoData font-mono" style="font-weight: bold">{{
            formatCoord(selectedDrone.latitude)
          }}</span>
        </div>
        <div class="infoRow">
          <span class="infoHeading">Lon:</span
          ><span class="infoData font-mono" style="font-weight: bold">{{
            formatCoord(selectedDrone.longitude)
          }}</span>
        </div>
      </div>

      <!-- 🎯 动态渲染后端返回的所有其他字段 -->
      <div
        class="infoBlockSection"
        style="border-top: 1px solid var(--BGCOLOR2); padding-top: 2px; padding-bottom: 2px"
      >
        <div v-for="item in detailEntries" :key="item.label" class="infoRow">
          <span class="infoHeading">{{ item.label }}:</span>
          <span class="infoData font-mono" style="font-size: 11px">{{ item.value }}</span>
        </div>

        <!-- 如果后端真的什么都没返回，显示占位符 -->
        <div
          v-if="detailEntries.length === 0"
          class="py-2 text-center"
          style="color: var(--TXTCOLOR2); font-size: 11px"
        >
          No additional details available
        </div>
      </div>

      <!-- 底部操作按钮 -->
      <div class="infoBlockSection" style="border-top: 1px solid var(--BGCOLOR2)">
        <button class="sidebarButton w-full" style="font-size: 11px" @click="exportDroneData">
          Export CSV
        </button>
      </div>
    </div>
    <!-- ========== 🆕 调试面板 1：实时统计 ========== -->
    <div v-if="showStatsModal" class="modal-overlay" @click.self="toggleStatsModal(false)">
      <div class="modal-content" style="width: 400px">
        <div class="modal-header">
          <span>📊 实时抓包统计 (每3秒刷新)</span>
          <button @click="toggleStatsModal(false)">✕</button>
        </div>
        <div v-if="statsData" class="modal-body">
          <div class="stat-grid">
            <div class="stat-item">
              <div class="stat-value">{{ statsData.total_captured || 0 }}</div>
              <div class="stat-label">总捕获包数</div>
            </div>
            <div class="stat-item">
              <div class="stat-value" style="color: #38a169">
                {{ statsData.parse_success || 0 }}
              </div>
              <div class="stat-label">解析成功</div>
            </div>
            <div class="stat-item">
              <div class="stat-value" style="color: #e53e3e">{{ statsData.parse_err || 0 }}</div>
              <div class="stat-label">解析失败</div>
            </div>
            <div class="stat-item">
              <div class="stat-value" style="color: #dd6b20">{{ statsData.oui_filtered || 0 }}</div>
              <div class="stat-label">OUI 过滤</div>
            </div>
          </div>
          <div class="stat-efficiency">
            <span>解析成功率:</span>
            <span class="efficiency-value">
              {{
                (
                  ((statsData.parse_success || 0) /
                    ((statsData.parse_success || 0) + (statsData.parse_err || 0) || 1)) *
                  100
                ).toFixed(1)
              }}%
            </span>
          </div>
          <div class="stat-efficiency" style="margin-top: 8px">
            <span>有效包占比 (成功/OUI过滤):</span>
            <span class="efficiency-value">
              {{
                (((statsData.parse_success || 0) / (statsData.oui_filtered || 1)) * 100).toFixed(1)
              }}%
            </span>
          </div>
        </div>
        <div v-else class="modal-body" style="text-align: center">加载中...</div>
      </div>
    </div>

    <!-- ========== 🆕 调试面板 2：即时抓包查看器 ========== -->
    <div v-if="showPacketsModal" class="modal-overlay" @click.self="showPacketsModal = false">
      <div
        class="modal-content"
        style="width: 90vw; max-width: 1200px; height: 80vh; display: flex; flex-direction: column"
      >
        <div class="modal-header">
          <span>📦 即时抓包查看器 (最近 50 条)</span>
          <div class="flex items-center gap-2">
            <select v-model="packetFilter" class="filter-select">
              <option value="all">所有阶段</option>
              <option value="captured">原始捕获</option>
              <option value="oui_filtered">OUI 过滤</option>
              <option value="parsed_ok">解析成功</option>
              <option value="parsed_err">解析失败</option>
            </select>
            <button class="refresh-btn" @click="fetchPackets">🔄 刷新</button>
            <button @click="showPacketsModal = false">✕</button>
          </div>
        </div>
        <div class="modal-body" style="flex: 1; overflow: auto; padding: 0">
          <table class="packet-table">
            <thead>
              <tr>
                <th style="width: 140px">MAC</th>
                <th style="width: 60px">RSSI</th>
                <th style="width: 100px">阶段 (Stage)</th>
                <th style="width: 120px">协议</th>
                <th style="width: 160px">时间</th>
                <th>Payload (Hex)</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(pkt, idx) in filteredPackets"
                :key="idx"
                :class="'pkt-stage-' + pkt.stage"
              >
                <td class="mono">{{ pkt.mac || '-' }}</td>
                <td class="mono">{{ pkt.rssi || '-' }}</td>
                <td>
                  <span class="stage-badge" :class="'badge-' + pkt.stage">{{ pkt.stage }}</span>
                </td>
                <td>{{ pkt.protocol || '-' }}</td>
                <td class="mono" style="font-size: 11px">{{ formatPacketTime(pkt.timestamp) }}</td>
                <td class="mono hex-cell" :title="pkt.hex">
                  {{ pkt.hex ? pkt.hex.substring(0, 80) + '...' : '-' }}
                </td>
              </tr>
              <tr v-if="filteredPackets.length === 0">
                <td colspan="6" style="text-align: center; padding: 20px; color: #999">暂无数据</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
    <!-- 侧边栏底部 -->
    <div id="sidebar_footer">
      <div class="flex flex-wrap gap-1">
        <button class="sidebarButton" title="Export CSV" @click="exportData">CSV</button>
        <button class="sidebarButton" title="Show all traces" @click="showAllTrajectories">
          Trk
        </button>
        <button class="sidebarButton" title="Clear traces" @click="clearTrajectories">Clr</button>

        <!-- 🆕 新增：调试面板触发按钮 -->
        <button
          class="sidebarButton"
          style="background: #2d3748; color: #fff"
          title="Debug Stats"
          @click="toggleStatsModal(true)"
        >
          Stats
        </button>
        <button
          class="sidebarButton"
          style="background: #2d3748; color: #fff"
          title="Packet Inspector"
          @click="openPacketsModal"
        >
          Pkts
        </button>
      </div>
      <span class="text-xs" style="color: var(--TXTCOLOR2)"
        >{{ store.alerts.filter((a) => a).length }} alerts</span
      >
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted, toRef } from 'vue'
import {
  timeAgo,
  isRecent,
  shortenMac,
  formatCoord,
  formatTime,
  downloadBlob,
} from '@/utils/helpers'
import { fetchDroneDetail, fetchDroneExport, dronesToCSV } from '@/utils/api'
import { useDroneStore } from '@/stores/drones'
import logger from '@/utils/logger'

const props = defineProps({
  initialWidth: { type: Number, default: 350 },
  // ❌ 删除 modelValue，改为 selectedDrone
  // modelValue: { type: Object, default: null }
  selectedDrone: { type: Object, default: null }, // ✅ 修正 Prop 名称
})

// ✅ 核心修复：将 prop 映射为模板可直接使用的变量
// 这样模板里所有的 selectedDrone 都能正常工作了！
const selectedDrone = toRef(props, 'selectedDrone')

const emit = defineEmits([
  // ❌ 删除 update:modelValue
  // 'update:modelValue',
  'update:selectedDrone', // ✅ 修正 Emit 名称
  'close',
  'show-trajectories',
  'clear-trajectories',
])

const store = useDroneStore()

// ---- 内部状态 ----
const width = ref(props.initialWidth)
const searchQuery = ref('')
const showInfoBlock = ref(false)

// ❌ 修改前：const localSelectedDrone = ref(props.modelValue)
const localSelectedDrone = ref(props.selectedDrone) // ✅ 修正

const selectedDroneDetail = ref(null)
const dataStaleWarning = ref(false)

let staleCheckInterval = null

// ❌ 修改前：watch(() => props.modelValue, (newVal) => {
watch(
  () => props.selectedDrone,
  (newVal) => {
    // ✅ 修正
    localSelectedDrone.value = newVal
    if (!newVal) {
      showInfoBlock.value = false
      selectedDroneDetail.value = null
    }
  },
)

// ---- 列选择配置 ----
const availableColumns = [
  { key: 'ua_type', label: 'Type', align: 'left', size: 'var(--FS2, 12px)', default: true },
  { key: 'standard', label: 'Std', align: 'left', size: 'var(--FS2, 12px)', default: true },
  { key: 'source', label: 'Via', align: 'left', size: '10px', default: false },
  { key: 'altitude', label: 'Alt', align: 'right', size: 'var(--FS2, 12px)', default: true },
  { key: 'latitude', label: 'Lat', align: 'right', size: 'var(--FS2, 12px)', default: true },
  { key: 'longitude', label: 'Lon', align: 'right', size: 'var(--FS2, 12px)', default: true },
  { key: 'speed', label: 'Speed', align: 'right', size: 'var(--FS2, 12px)', default: false },
  { key: 'heading', label: 'Hdg', align: 'right', size: 'var(--FS2, 12px)', default: false },
  { key: 'status', label: 'Status', align: 'left', size: 'var(--FS2, 12px)', default: false },
  { key: 'id_type', label: 'ID Type', align: 'left', size: '10px', default: false },
  { key: 'signal', label: 'Signal', align: 'left', size: 'var(--FS2, 12px)', default: false },
  { key: 'first_seen', label: 'First', align: 'right', size: '10px', default: false },
  { key: 'operator_lat', label: 'Op Lat', align: 'right', size: '10px', default: false },
  { key: 'operator_lng', label: 'Op Lng', align: 'right', size: '10px', default: false },
  { key: 'area_radius', label: 'Radius', align: 'right', size: '10px', default: false },
  { key: 'region', label: 'Region', align: 'left', size: '10px', default: false },
]

const visibleColumns = ref(Object.fromEntries(availableColumns.map((c) => [c.key, c.default])))
const visibleColumnList = computed(() =>
  availableColumns.filter((c) => visibleColumns.value[c.key]),
)

// ---- 计算属性 ----
const filteredDrones = computed(() => {
  let drones = store.activeDrones
  const q = searchQuery.value.toLowerCase().trim()
  if (q) {
    drones = drones.filter(
      (d) =>
        (d.uas_id || '').toLowerCase().includes(q) ||
        (d.mac || '').toLowerCase().includes(q) ||
        (d.ua_type || '').toLowerCase().includes(q),
    )
  }
  return drones
})

// ---- 动态解析无人机所有详情字段 ----
const detailEntries = computed(() => {
  // 合并基础列表数据和 API 拉取的详情数据
  const data = { ...selectedDrone.value, ...selectedDroneDetail.value }
  if (!data || Object.keys(data).length === 0) return []

  // 字段映射与格式化规则 (将后端 snake_case 转为人类可读标签，并格式化数值)
  const fieldConfig = {
    protocol: { label: 'Protocol', format: (v) => v || 'N/A' },
    uas_id: { label: 'UAS ID', format: (v) => v || 'N/A' },
    ua_type: { label: 'UA Type', format: (v) => v || 'N/A' },
    standard: { label: 'Standard', format: (v) => v || 'N/A' },
    source: { label: 'Source', format: (v) => v || 'N/A' },
    id_type: { label: 'ID Type', format: (v) => v || 'N/A' },
    operator_id: { label: 'Operator ID', format: (v) => v || 'N/A' },
    altitude: {
      label: 'Altitude (Geo)',
      format: (v) => (v != null ? `${parseFloat(v).toFixed(1)} m` : 'N/A'),
    },
    height: {
      label: 'Height (AGL)',
      format: (v) => (v != null ? `${parseFloat(v).toFixed(1)} m` : 'N/A'),
    },
    speed: {
      label: 'Speed (H)',
      format: (v) => (v != null ? `${parseFloat(v).toFixed(1)} m/s` : 'N/A'),
    },
    speed_v: {
      label: 'Speed (V)',
      format: (v) => (v != null ? `${parseFloat(v).toFixed(1)} m/s` : 'N/A'),
    },
    heading: {
      label: 'Heading',
      format: (v) => (v != null ? `${parseFloat(v).toFixed(0)}°` : 'N/A'),
    },
    flight_status: { label: 'Flight Status', format: (v) => v || 'N/A' },
    height_type: { label: 'Height Type', format: (v) => v || 'N/A' },
    signal_strength: { label: 'Signal', format: (v) => v || 'N/A' },
    rssi: { label: 'RSSI', format: (v) => (v != null ? `${v} dBm` : 'N/A') },
    battery_level: { label: 'Battery', format: (v) => v || 'N/A' },
    flight_time: { label: 'Flight Time', format: (v) => v || 'N/A' },
    operator_latitude: {
      label: 'Operator Lat',
      format: (v) => (v != null ? formatCoord(v) : 'N/A'),
    },
    operator_longitude: {
      label: 'Operator Lng',
      format: (v) => (v != null ? formatCoord(v) : 'N/A'),
    },
    area_radius_m: { label: 'Area Radius', format: (v) => (v != null ? `${v} m` : 'N/A') },
    classification_region: { label: 'Region', format: (v) => v || 'N/A' },
    h_accuracy: { label: 'H Accuracy', format: (v) => v || 'N/A' },
    v_accuracy: { label: 'V Accuracy', format: (v) => v || 'N/A' },
    s_accuracy: { label: 'S Accuracy', format: (v) => v || 'N/A' },
    first_seen: { label: 'First Seen', format: (v) => formatTime(v) },
    last_seen: { label: 'Last Seen', format: (v) => timeAgo(v) },
  }

  // 排除已经在顶部 Header 和基础坐标区单独展示的字段
  const excludeKeys = ['mac', 'latitude', 'longitude']

  const entries = []
  for (const [key, value] of Object.entries(data)) {
    if (excludeKeys.includes(key)) continue

    // 如果配置表里有，用配置表的 label 和 format
    if (fieldConfig[key]) {
      entries.push({
        label: fieldConfig[key].label,
        value: fieldConfig[key].format(value),
      })
    } else {
      // 🎯 兜底：后端返回了未知的新字段，自动将 snake_case 转为 Title Case 并显示
      const prettyKey = key.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase())
      entries.push({
        label: prettyKey,
        value: value != null ? String(value) : 'N/A',
      })
    }
  }
  return entries
})
// ---- 🆕 调试面板状态与逻辑 ----
const showStatsModal = ref(false)
const statsData = ref(null)
let statsInterval = null

const showPacketsModal = ref(false)
const packetsData = ref([])
const packetFilter = ref('all')

// 过滤抓包数据
const filteredPackets = computed(() => {
  if (packetFilter.value === 'all') return packetsData.value
  return packetsData.value.filter((p) => p.stage === packetFilter.value)
})

// 切换统计面板
const toggleStatsModal = (val) => {
  showStatsModal.value = val
  if (val) {
    fetchStats()
    statsInterval = setInterval(fetchStats, 3000) // 每 3 秒轮询
  } else {
    if (statsInterval) clearInterval(statsInterval)
  }
}

// 获取统计数据
const fetchStats = async () => {
  try {
    const res = await fetch('/api/debug/stats')
    if (res.ok) statsData.value = await res.json()
  } catch (e) {
    logger.error('Fetch stats error:', e)
  }
}

// 打开抓包查看器
const openPacketsModal = () => {
  showPacketsModal.value = true
  fetchPackets()
}

// 获取抓包数据
const fetchPackets = async () => {
  try {
    const res = await fetch('/api/debug/packets?limit=50')
    if (res.ok) packetsData.value = await res.json()
  } catch (e) {
    logger.error('Fetch packets error:', e)
  }
}

// 格式化时间戳
const formatPacketTime = (ts) => {
  if (!ts) return '-'
  const d = new Date(ts)
  if (isNaN(d.getTime())) return ts
  return d.toLocaleTimeString() + '.' + String(d.getMilliseconds()).padStart(3, '0')
}
// ---- 辅助方法 ----
const getRowClass = (drone) => {
  if (!drone) return ''
  const classes = []
  if (!isRecent(drone.last_seen, 5)) classes.push('stale_row')
  if (drone.standard === 'ASTM F3411-22a') classes.push('astm_row')
  return classes.join(' ')
}

const timeAgoShort = (ts) => {
  if (!ts) return '-'
  const diff = Math.floor((Date.now() - new Date(ts).getTime()) / 1000)
  if (diff < 60) return diff + 's'
  if (diff < 3600) return Math.floor(diff / 60) + 'm'
  if (diff < 86400) return Math.floor(diff / 3600) + 'h'
  return Math.floor(diff / 86400) + 'd'
}

const formatTimeShort = (ts) => {
  if (!ts) return '-'
  const d = new Date(ts)
  if (isNaN(d.getTime())) return '-'
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

// ---- 核心交互逻辑 ----
const closeInfoBlock = () => {
  showInfoBlock.value = false
  // ❌ 修改前：emit('update:modelValue', null)
  emit('update:selectedDrone', null) // ✅ 修正 (在 closeInfoBlock 函数中)
  selectedDroneDetail.value = null
}

// ---- 无人机选择 ----
// ---- 在 Sidebar.vue 中 ----
const selectDrone = async (drone) => {
  // ✅ 1. 核心修复：通过 emit 通知父组件更新状态，绝不直接修改 prop！
  emit('update:selectedDrone', drone)

  // 2. 展开侧边栏的详情面板
  showInfoBlock.value = true

  // 3. 更新本地用于展示详情的变量 (避免修改 readonly 的 prop)
  localSelectedDrone.value = drone

  // 4. 获取并合并详情数据
  try {
    const detail = await fetchDroneDetail(drone.mac)
    selectedDroneDetail.value = detail
    // 将完整详情合并到本地变量中，供模板渲染使用
    localSelectedDrone.value = { ...drone, ...detail }
  } catch (e) {
    logger.error('Fetch drone detail error:', e)
  }

  // ❌ 彻底删除以下代码！Sidebar 里没有 map 实例！
  // if (map.value && drone.latitude && drone.longitude) {
  //   map.value.flyTo(...)
  // }
}

const exportData = async () => {
  try {
    if (!store.activeDrones.length) return
    const allData = []
    for (const d of store.activeDrones) {
      try {
        allData.push(await fetchDroneExport(d.mac))
      } catch (e) {
        // 忽略导出错误，或记录日志
        logger.error('Export error:', e)
      }
    }
    const csv = dronesToCSV(allData)
    downloadBlob(
      new Blob([csv], { type: 'text/csv;charset=utf-8;' }),
      `all_drones_${new Date().toISOString().split('T')[0]}.csv`,
    )
  } catch (e) {
    logger.error('Export error:', e)
  }
}

const exportDroneData = async () => {
  if (!localSelectedDrone.value) return
  try {
    const data = await fetchDroneExport(localSelectedDrone.value.mac)
    const csv = dronesToCSV([data])
    downloadBlob(
      new Blob([csv], { type: 'text/csv;charset=utf-8;' }),
      `drone_${localSelectedDrone.value.mac.replace(/:/g, '_')}_${new Date().toISOString().split('T')[0]}.csv`,
    )
  } catch (e) {
    logger.error('Export error:', e)
  }
}

// ---- 数据新鲜度监控 ----
const checkDataStaleness = () => {
  const droneList = store.activeDrones
  if (droneList.length > 0) {
    const latestSeen = Math.max(...droneList.map((d) => new Date(d.last_seen).getTime()))
    dataStaleWarning.value = Date.now() - latestSeen > 60000
  } else {
    dataStaleWarning.value = false
  }
}

// ---- 生命周期 ----
onMounted(() => {
  checkDataStaleness()
  staleCheckInterval = setInterval(checkDataStaleness, 5000)
})

onUnmounted(() => {
  if (staleCheckInterval) clearInterval(staleCheckInterval)

  // 🆕 加上这一行，清理统计面板的轮询定时器
  if (statsInterval) clearInterval(statsInterval)
})
</script>

<style scoped>
/* ---- 右侧边栏容器 ---- */
#sidebar_container {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  height: 100%;
  overflow: hidden;
  border-left: 1px solid var(--BGCOLOR2, #ccc);
  background: var(--BGCOLOR1, #fff);
  transition: width 0.1s ease-out;
}

#sidebar_header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 8px;
  flex-shrink: 0;
  background-color: var(--ACCENT, #3182ce);
  color: #fff;
}

#header_title {
  color: #fff;
  font-weight: bold;
  font-size: var(--FS2, 12px);
}

#header_count {
  font-size: var(--FS1, 11px);
  opacity: 0.8;
}

.stale-warning-bar {
  background: #e53e3e;
  color: #fff;
  text-align: center;
  padding: 4px;
  font-size: 11px;
  flex-shrink: 0;
}

/* 搜索栏 */
#search_bar {
  padding: 4px 6px;
  flex-shrink: 0;
  background: var(--BGCOLOR1, #fff);
  border-bottom: 1px solid var(--BGCOLOR2, #ccc);
}

#search_input {
  width: 100%;
  height: 22px;
  padding: 0 4px;
  font-size: var(--FS1, 11px);
  background: #fff;
  border: 1px solid var(--BGCOLOR2, #ccc);
  outline: none;
  border-radius: 2px;
}

#search_input:focus {
  border-color: var(--ACCENT, #3182ce);
}

/* 侧边栏表格区域 */
#sidebar_canvas {
  flex: 1 1 0;
  overflow-y: auto;
  overflow-x: auto;
  min-height: 0;
}

#planesTable {
  width: 100%;
  font-size: var(--FS2, 12px);
  white-space: nowrap;
  border-collapse: collapse;
}

.aircraft_table_header th {
  position: sticky;
  top: 0;
  z-index: 10;
  background-color: var(--ACCENT, #3182ce);
  color: #fff;
  font-weight: normal;
  padding: 4px 4px;
  box-shadow: 0 2px 2px -1px rgba(0, 0, 0, 0.5);
  text-align: left;
}

#planesTable td {
  padding: 2px 4px;
  cursor: pointer;
}

.icaoCodeColumn {
  font-family: monospace;
  text-transform: uppercase;
}

/* 表格行颜色 */
.plane_table_row:hover {
  background-color: rgba(0, 0, 0, 0.05);
}

.plane_table_row.stale_row {
  opacity: 0.45;
}

.plane_table_row.astm_row {
  background-color: #e0f0ff;
}

/* 侧边栏底部 */
#sidebar_footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 6px;
  flex-shrink: 0;
  border-top: 1px solid var(--BGCOLOR2, #ccc);
  background: var(--BGCOLOR1, #fff);
}

/* ---- 侧边栏内无人机详情 ---- */
#sidebar_drone_detail {
  flex-shrink: 0;
  max-height: 45vh;
  overflow-y: auto;
  border-top: 1px solid var(--BGCOLOR2, #ccc);
  background: var(--BGCOLOR1, #fff);
  font-size: var(--FS2, 12px);
}

.infoRow {
  clear: both;
  padding: 1px 0;
}

.infoHeading {
  display: inline;
  margin-right: 4px;
  color: var(--TXTCOLOR2, #666);
}

.infoData {
  float: right;
  text-align: right;
  color: var(--TXTCOLOR1, #333);
}

.infoBlockSection {
  padding: 6px 8px;
}

/* 状态点样式 (需在 global css 中定义，这里提供 fallback) */
.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
}
.status-dot.live {
  background-color: #48bb78;
  box-shadow: 0 0 4px #48bb78;
}
.status-dot.error {
  background-color: #f56565;
  box-shadow: 0 0 4px #f56565;
  animation: pulse 1.5s infinite;
}

@keyframes pulse {
  0% {
    opacity: 1;
  }
  50% {
    opacity: 0.5;
  }
  100% {
    opacity: 1;
  }
}

/* =========================================================================
🆕 调试面板 Modal 样式
========================================================================= */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  z-index: 10000;
  display: flex;
  align-items: center;
  justify-content: center;
  backdrop-filter: blur(2px);
}
.modal-content {
  background: var(--BGCOLOR1);
  border-radius: 6px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.5);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--BGCOLOR2);
}
.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  background: var(--ACCENT);
  color: #fff;
  font-weight: bold;
  font-size: 14px;
}
.modal-header button {
  background: transparent;
  border: none;
  color: #fff;
  font-size: 18px;
  cursor: pointer;
  opacity: 0.8;
}
.modal-header button:hover {
  opacity: 1;
}
.modal-body {
  padding: 16px;
  overflow-y: auto;
}

/* ---- Stats 面板 ---- */
.stat-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-bottom: 16px;
}
.stat-item {
  background: #f8f9fa;
  padding: 12px;
  border-radius: 4px;
  text-align: center;
  border: 1px solid #e2e8f0;
}
.stat-value {
  font-size: 24px;
  font-weight: bold;
  font-family: monospace;
}
.stat-label {
  font-size: 12px;
  color: #666;
  margin-top: 4px;
}
.stat-efficiency {
  display: flex;
  justify-content: space-between;
  font-size: 14px;
  padding: 8px 12px;
  background: #edf2f7;
  border-radius: 4px;
}
.efficiency-value {
  font-weight: bold;
  font-family: monospace;
  color: var(--ACCENT);
}

/* ---- Packets 面板 ---- */
.filter-select {
  padding: 4px 8px;
  border-radius: 4px;
  border: 1px solid rgba(255, 255, 255, 0.3);
  background: rgba(255, 255, 255, 0.1);
  color: #fff;
  font-size: 12px;
  outline: none;
}
.filter-select option {
  color: #000;
}
.refresh-btn {
  background: rgba(255, 255, 255, 0.2);
  border: none;
  color: #fff;
  padding: 4px 8px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
}
.packet-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}
.packet-table th {
  position: sticky;
  top: 0;
  background: #2d3748;
  color: #fff;
  padding: 8px;
  text-align: left;
  font-weight: normal;
}
.packet-table td {
  padding: 6px 8px;
  border-bottom: 1px solid #e2e8f0;
  vertical-align: top;
}
.packet-table tr:hover {
  background: #f7fafc;
}
.mono {
  font-family: monospace;
}
.hex-cell {
  max-width: 400px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #4a5568;
  font-size: 11px;
}
.stage-badge {
  padding: 2px 6px;
  border-radius: 10px;
  font-size: 10px;
  font-weight: bold;
  color: #fff;
}
.badge-captured {
  background: #718096;
}
.badge-oui_filtered {
  background: #dd6b20;
}
.badge-parsed_ok {
  background: #38a169;
}
.badge-parsed_err {
  background: #e53e3e;
}

/* 行左侧高亮指示条 */
.pkt-stage-parsed_ok {
  border-left: 3px solid #38a169;
}
.pkt-stage-parsed_err {
  border-left: 3px solid #e53e3e;
}
</style>
