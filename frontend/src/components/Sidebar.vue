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
        <button @click="$emit('close')" class="text-white opacity-60 hover:opacity-100" title="Hide sidebar" style="font-size:14px; line-height:1;">✕</button>
      </div>
    </div>

    <!-- 数据过期警告条 -->
    <div v-if="dataStaleWarning" class="stale-warning-bar">
      ⚠ 数据已超过 60 秒未更新 — 请检查后端抓包服务是否正常运行
    </div>

    <!-- 搜索栏 -->
    <div id="search_bar">
      <input
        v-model="searchQuery"
        type="text"
        placeholder="Filter by ID/MAC/Type..."
        id="search_input"
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
            >{{ col.label }}</th>
            <th class="text-right font-mono">Seen</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="drone in filteredDrones"
            :key="drone.mac"
            class="plane_table_row"
            :class="getRowClass(drone)"
            @click="selectDrone(drone)"
            :style="selectedDrone?.mac === drone.mac ? 'background-color: var(--ACCENT, #3182ce); color: #fff;' : ''"
          >
            <td class="icaoCodeColumn">
              <div class="font-bold">{{ drone.uas_id || shortenMac(drone.mac, false) }}</div>
              <div class="opacity-50" style="font-size:9px;">{{ drone.mac }}</div>
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
              <template v-else-if="col.key === 'altitude'">{{ drone.altitude != null ? Math.round(drone.altitude) + 'm' : '-' }}</template>
              <template v-else-if="col.key === 'latitude'">{{ drone.latitude != null ? drone.latitude.toFixed(4) : '-' }}</template>
              <template v-else-if="col.key === 'longitude'">{{ drone.longitude != null ? drone.longitude.toFixed(4) : '-' }}</template>
              <template v-else-if="col.key === 'speed'">{{ drone.speed != null ? drone.speed.toFixed(1) + 'm/s' : '-' }}</template>
              <template v-else-if="col.key === 'heading'">{{ drone.heading != null ? drone.heading + '°' : '-' }}</template>
              <template v-else-if="col.key === 'status'">{{ drone.status || '-' }}</template>
              <template v-else-if="col.key === 'id_type'">{{ drone.id_type || '-' }}</template>
              <template v-else-if="col.key === 'signal'">{{ drone.signal_strength || '-' }}</template>
              <template v-else-if="col.key === 'first_seen'">{{ formatTimeShort(drone.first_seen) }}</template>
              <template v-else-if="col.key === 'operator_lat'">{{ drone.operator_latitude != null ? drone.operator_latitude.toFixed(4) : '-' }}</template>
              <template v-else-if="col.key === 'operator_lng'">{{ drone.operator_longitude != null ? drone.operator_longitude.toFixed(4) : '-' }}</template>
              <template v-else-if="col.key === 'area_radius'">{{ drone.area_radius_m != null ? drone.area_radius_m + 'm' : '-' }}</template>
              <template v-else-if="col.key === 'region'">{{ drone.classification_region || '-' }}</template>
              <template v-else>-</template>
            </td>
            <td class="text-right font-mono" style="font-size: var(--FS2, 12px);">{{ timeAgoShort(drone.last_seen) }}</td>
          </tr>
          <tr v-if="filteredDrones.length === 0">
            <td :colspan="2 + visibleColumnList.length" class="text-center py-4" style="color: var(--BGCOLOR2, #999);">No drones detected</td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 选中无人机详情（侧边栏内，表格下方） -->
<!-- 选中无人机详情（侧边栏内，表格下方） -->
<div v-if="showInfoBlock && selectedDrone" id="sidebar_drone_detail">
  <!-- 顶部 Header -->
  <div class="flex items-center justify-between px-2 py-1 text-xs font-bold text-white" style="background-color: var(--ACCENT);">
    <span>{{ selectedDrone.uas_id || 'Unknown UAS' }}</span>
    <button @click="closeInfoBlock" class="text-white text-xs">✕</button>
  </div>
  
  <!-- MAC 地址 -->
  <div class="infoBlockSection" style="padding-bottom:2px;">
    <div class="font-mono uppercase" style="font-size:10px; color: var(--TXTCOLOR2); opacity:0.7;">{{ selectedDrone.mac }}</div>
  </div>

  <!-- 核心坐标区 (保持醒目) -->
  <div class="infoBlockSection" style="border-top:1px solid var(--BGCOLOR2); padding-top:4px; padding-bottom:4px; background: rgba(0,0,0,0.02);">
    <div class="infoRow"><span class="infoHeading">Lat:</span><span class="infoData font-mono" style="font-weight:bold;">{{ formatCoord(selectedDrone.latitude) }}</span></div>
    <div class="infoRow"><span class="infoHeading">Lon:</span><span class="infoData font-mono" style="font-weight:bold;">{{ formatCoord(selectedDrone.longitude) }}</span></div>
  </div>

  <!-- 🎯 动态渲染后端返回的所有其他字段 -->
  <div class="infoBlockSection" style="border-top:1px solid var(--BGCOLOR2); padding-top:2px; padding-bottom:2px;">
    <div v-for="item in detailEntries" :key="item.label" class="infoRow">
      <span class="infoHeading">{{ item.label }}:</span>
      <span class="infoData font-mono" style="font-size:11px;">{{ item.value }}</span>
    </div>
    
    <!-- 如果后端真的什么都没返回，显示占位符 -->
    <div v-if="detailEntries.length === 0" class="text-center py-2" style="color: var(--TXTCOLOR2); font-size: 11px;">
      No additional details available
    </div>
  </div>

  <!-- 底部操作按钮 -->
  <div class="infoBlockSection" style="border-top:1px solid var(--BGCOLOR2);">
    <button @click="goToDetail" class="sidebarButton w-full mb-1" style="font-size:11px;">View Full Details</button>
    <button @click="exportDroneData" class="sidebarButton w-full" style="font-size:11px;">Export CSV</button>
  </div>
</div>

    <!-- 侧边栏底部 -->
    <div id="sidebar_footer">
      <div class="flex gap-1">
        <button @click="exportData" class="sidebarButton" title="Export CSV">CSV</button>
        <button @click="$emit('show-trajectories')" class="sidebarButton" title="Show all traces">Trk</button>
        <button @click="$emit('clear-trajectories')" class="sidebarButton" title="Clear traces">Clr</button>
      </div>
      <span class="text-xs" style="color: var(--TXTCOLOR2, #666);">{{ store.alerts.filter(a => a).length }} alerts</span>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted, toRef } from 'vue'
import { timeAgo, isRecent, shortenMac, formatCoord, formatTime, downloadBlob } from '@/utils/helpers'
import { fetchDroneDetail, fetchDroneExport, dronesToCSV } from '@/utils/api'
import { useDroneStore } from '@/stores/drones'
import logger from '@/utils/logger'

const props = defineProps({
  initialWidth: { type: Number, default: 350 },
  // ❌ 删除 modelValue，改为 selectedDrone
  // modelValue: { type: Object, default: null } 
  selectedDrone: { type: Object, default: null } // ✅ 修正 Prop 名称
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
  'clear-trajectories'
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
watch(() => props.selectedDrone, (newVal) => { // ✅ 修正
  localSelectedDrone.value = newVal
  if (!newVal) {
    showInfoBlock.value = false
    selectedDroneDetail.value = null
  }
})


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
  { key: 'region', label: 'Region', align: 'left', size: '10px', default: false }
]

const visibleColumns = ref(Object.fromEntries(availableColumns.map(c => [c.key, c.default])))
const visibleColumnList = computed(() => availableColumns.filter(c => visibleColumns.value[c.key]))

// ---- 计算属性 ----
const filteredDrones = computed(() => {
  let drones = store.activeDrones
  const q = searchQuery.value.toLowerCase().trim()
  if (q) {
    drones = drones.filter(d =>
      (d.uas_id || '').toLowerCase().includes(q) ||
      (d.mac || '').toLowerCase().includes(q) ||
      (d.ua_type || '').toLowerCase().includes(q)
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
    protocol: { label: 'Protocol', format: v => v || 'N/A' },
    uas_id: { label: 'UAS ID', format: v => v || 'N/A' },
    ua_type: { label: 'UA Type', format: v => v || 'N/A' },
    standard: { label: 'Standard', format: v => v || 'N/A' },
    source: { label: 'Source', format: v => v || 'N/A' },
    id_type: { label: 'ID Type', format: v => v || 'N/A' },
    operator_id: { label: 'Operator ID', format: v => v || 'N/A' },
    altitude: { label: 'Altitude (Geo)', format: v => v != null ? `${parseFloat(v).toFixed(1)} m` : 'N/A' },
    height: { label: 'Height (AGL)', format: v => v != null ? `${parseFloat(v).toFixed(1)} m` : 'N/A' },
    speed: { label: 'Speed (H)', format: v => v != null ? `${parseFloat(v).toFixed(1)} m/s` : 'N/A' },
    speed_v: { label: 'Speed (V)', format: v => v != null ? `${parseFloat(v).toFixed(1)} m/s` : 'N/A' },
    heading: { label: 'Heading', format: v => v != null ? `${parseFloat(v).toFixed(0)}°` : 'N/A' },
    flight_status: { label: 'Flight Status', format: v => v || 'N/A' },
    height_type: { label: 'Height Type', format: v => v || 'N/A' },
    signal_strength: { label: 'Signal', format: v => v || 'N/A' },
    rssi: { label: 'RSSI', format: v => v != null ? `${v} dBm` : 'N/A' },
    battery_level: { label: 'Battery', format: v => v || 'N/A' },
    flight_time: { label: 'Flight Time', format: v => v || 'N/A' },
    operator_latitude: { label: 'Operator Lat', format: v => v != null ? formatCoord(v) : 'N/A' },
    operator_longitude: { label: 'Operator Lng', format: v => v != null ? formatCoord(v) : 'N/A' },
    area_radius_m: { label: 'Area Radius', format: v => v != null ? `${v} m` : 'N/A' },
    classification_region: { label: 'Region', format: v => v || 'N/A' },
    h_accuracy: { label: 'H Accuracy', format: v => v || 'N/A' },
    v_accuracy: { label: 'V Accuracy', format: v => v || 'N/A' },
    s_accuracy: { label: 'S Accuracy', format: v => v || 'N/A' },
    first_seen: { label: 'First Seen', format: v => formatTime(v) },
    last_seen: { label: 'Last Seen', format: v => timeAgo(v) },
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
        value: fieldConfig[key].format(value)
      })
    } else {
      // 🎯 兜底：后端返回了未知的新字段，自动将 snake_case 转为 Title Case 并显示
      const prettyKey = key.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase())
      entries.push({
        label: prettyKey,
        value: value != null ? String(value) : 'N/A'
      })
    }
  }
  return entries
})

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

// ✅ 新增：在地图上居中并打开弹窗
const focusOnMap = () => {
  if (selectedDrone.value && map.value) {
    const { latitude, longitude, mac } = selectedDrone.value
    if (latitude && longitude) {
      map.value.setView([latitude, longitude], 16) // 缩放到 16 级
      // 打开对应的 Marker 弹窗
      if (droneMarkers.value[mac]) {
        droneMarkers.value[mac].openPopup()
      }
    }
  }
}

const exportData = async () => {
  try {
    if (!store.activeDrones.length) return
    const allData = []
    for (const d of store.activeDrones) {
      try { allData.push(await fetchDroneExport(d.mac)) } catch (e) {}
    }
    const csv = dronesToCSV(allData)
    downloadBlob(new Blob([csv], { type: 'text/csv;charset=utf-8;' }),
      `all_drones_${new Date().toISOString().split('T')[0]}.csv`)
  } catch (e) { logger.error('Export error:', e) }
}

const exportDroneData = async () => {
  if (!localSelectedDrone.value) return
  try {
    const data = await fetchDroneExport(localSelectedDrone.value.mac)
    const csv = dronesToCSV([data])
    downloadBlob(new Blob([csv], { type: 'text/csv;charset=utf-8;' }),
      `drone_${localSelectedDrone.value.mac.replace(/:/g,'_')}_${new Date().toISOString().split('T')[0]}.csv`)
  } catch (e) { logger.error('Export error:', e) }
}

// ---- 数据新鲜度监控 ----
const checkDataStaleness = () => {
  const droneList = store.activeDrones
  if (droneList.length > 0) {
    const latestSeen = Math.max(...droneList.map(d => new Date(d.last_seen).getTime()))
    dataStaleWarning.value = (Date.now() - latestSeen) > 60000
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
  color: #FFF;
}

#header_title {
  color: #FFF;
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
  color: #FFF;
  font-weight: normal;
  padding: 4px 4px;
  box-shadow: 0 2px 2px -1px rgba(0,0,0,.5);
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
  background-color: rgba(0,0,0,0.05);
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
  0% { opacity: 1; }
  50% { opacity: 0.5; }
  100% { opacity: 1; }
}
</style>