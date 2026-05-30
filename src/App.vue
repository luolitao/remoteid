<!-- src/App.vue — sidebar(right) + map(left), both independently hideable -->
<template>
  <div id="layout_container">
    <!-- ========== 地图区域（左侧主体） ========== -->
    <div id="map_container" v-show="mapOpen">
      <div id="map_canvas" ref="mapContainer"></div>

      <!-- 左上角按钮组 -->
      <div id="header_side">
        <!-- 地图隐藏 -->
        <button
          @click="mapOpen = false"
          class="toggle_sidebar_button"
          title="Hide map"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <rect x="3" y="3" width="18" height="18" rx="2" />
            <line x1="9" y1="3" x2="9" y2="21" />
          </svg>
        </button>

        <!-- 地图类型切换 -->
        <div class="flex gap-0.5 ml-2">
          <button
            v-for="mt in mapTypeOptions"
            :key="mt.key"
            @click="changeMapType(mt.key)"
            class="sidebarButton"
            :style="currentMapType === mt.key ? 'background-color: var(--ACCENT);' : ''"
            style="font-size:11px; padding:2px 6px;"
          >{{ mt.label }}</button>
        </div>

        <!-- 设置齿轮 -->
        <button
          @click="showSettings = !showSettings"
          class="ml-2 flex items-center justify-center"
          style="width:32px; height:38px; border-radius:2px; background:rgba(255,255,255,.85); border:1px solid var(--BGCOLOR2);"
          title="Settings"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="3"/>
            <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
          </svg>
        </button>
      </div>

      <!-- ========== 设置面板 (地图相关设置) ========== -->
      <div
        v-if="showSettings"
        id="settings_infoblock"
      >
        <div id="settings_header">
          <span style="font-size: var(--FS3); font-weight: bold;">Map Settings</span>
          <button @click="showSettings = false" class="text-white" style="font-size:16px; line-height:1;">✕</button>
        </div>

        <div id="settings_content">
          <div class="settingsColumn">
            <div class="settingsSectionTitle">Map Layers</div>
            <label v-for="mt in mapTypeOptions" :key="mt.key" class="settingsCheckboxLabel" style="cursor:pointer;" @click="changeMapType(mt.key)">
              <input type="radio" :checked="currentMapType === mt.key" class="mr-1" style="width:12px;height:12px;" /> {{ mt.label }}
            </label>
          </div>

          <div class="settingsColumn">
            <div class="settingsSectionTitle">Panels</div>
            <label class="settingsCheckboxLabel" @click="sidebarOpen = !sidebarOpen">
              <input type="checkbox" :checked="sidebarOpen" class="mr-1" style="width:12px;height:12px;" /> Sidebar
            </label>
            <label class="settingsCheckboxLabel" @click="mapOpen = !mapOpen">
              <input type="checkbox" :checked="mapOpen" class="mr-1" style="width:12px;height:12px;" /> Map
            </label>
            <label class="settingsCheckboxLabel" style="cursor:pointer;" @click="showColumnPanel = !showColumnPanel">
              <input type="checkbox" :checked="showColumnPanel" class="mr-1" style="width:12px;height:12px;" /> Column Selector
            </label>
          </div>
        </div>
      </div>

      <!-- ========== 选中无人机信息块 ========== -->
      <div
        v-if="showInfoBlock && selectedDrone"
        id="selected_infoblock"
      >
        <div id="infoblock_close" @click="closeInfoBlock">✕</div>
        <div class="infoBlockSection" style="padding-bottom:2px;">
          <div class="identLarge">{{ selectedDrone.uas_id || 'Unknown' }}</div>
          <div class="font-mono uppercase" style="font-size:10px; color: var(--TXTCOLOR2); opacity:0.7;">{{ selectedDrone.mac }}</div>
        </div>
        <div class="infoBlockSection" style="padding-top:2px; padding-bottom:2px;">
          <div class="infoRow"><span class="infoHeading">Type:</span><span class="infoData">{{ selectedDrone.ua_type || 'N/A' }}</span></div>
          <div class="infoRow"><span class="infoHeading">Standard:</span><span class="infoData">{{ selectedDrone.standard || 'N/A' }}</span></div>
          <div class="infoRow">
            <span class="infoHeading">Compliance:</span>
            <span class="infoData" :style="{ color: selectedDrone.china_compliant ? '#38a169' : '#e53e3e' }">
              {{ selectedDrone.china_compliant ? 'CAA Compliant' : 'Non-compliant' }}
            </span>
          </div>
        </div>
        <div class="infoBlockSection" style="border-top:1px solid var(--BGCOLOR2); padding-top:2px; padding-bottom:2px;">
          <div class="infoRow"><span class="infoHeading">Lat:</span><span class="infoData font-mono">{{ formatCoord(selectedDrone.latitude) }}</span></div>
          <div class="infoRow"><span class="infoHeading">Lon:</span><span class="infoData font-mono">{{ formatCoord(selectedDrone.longitude) }}</span></div>
          <div class="infoRow"><span class="infoHeading">Alt:</span><span class="infoData font-mono">{{ selectedDrone.altitude ? selectedDrone.altitude.toFixed(0) + ' m' : 'N/A' }}</span></div>
          <div class="infoRow"><span class="infoHeading">Speed:</span><span class="infoData font-mono">{{ selectedDrone.speed ? selectedDrone.speed.toFixed(1) + ' m/s' : 'N/A' }}</span></div>
          <div class="infoRow"><span class="infoHeading">Heading:</span><span class="infoData font-mono">{{ selectedDrone.heading != null ? selectedDrone.heading + '°' : 'N/A' }}</span></div>
          <div v-if="selectedDrone.status" class="infoRow"><span class="infoHeading">Status:</span><span class="infoData">{{ selectedDrone.status }}</span></div>
          <div class="infoRow"><span class="infoHeading">Signal:</span><span class="infoData">{{ selectedDrone.signal_strength || 'N/A' }}</span></div>
          <div v-if="selectedDrone.battery_level" class="infoRow"><span class="infoHeading">Battery:</span><span class="infoData">{{ selectedDrone.battery_level }}</span></div>
          <div v-if="selectedDrone.flight_time" class="infoRow"><span class="infoHeading">Flight:</span><span class="infoData">{{ selectedDrone.flight_time }}</span></div>
        </div>
        <div class="infoBlockSection" style="border-top:1px solid var(--BGCOLOR2); padding-top:2px; padding-bottom:2px;">
          <div class="infoRow"><span class="infoHeading">First:</span><span class="infoData">{{ formatTime(selectedDrone.first_seen) }}</span></div>
          <div class="infoRow"><span class="infoHeading">Last:</span><span class="infoData">{{ timeAgo(selectedDrone.last_seen) }}</span></div>
        </div>
        <div v-if="selectedDrone.operator_latitude" class="infoBlockSection" style="border-top:1px solid var(--BGCOLOR2); padding-top:2px; padding-bottom:2px;">
          <div style="font-weight:bold; font-size:11px; margin-bottom:2px; color:var(--TXTCOLOR1);">Operator</div>
          <div class="infoRow"><span class="infoHeading">Pos:</span><span class="infoData font-mono" style="font-size:10px;">{{ formatCoord(selectedDrone.operator_latitude) }}, {{ formatCoord(selectedDrone.operator_longitude) }}</span></div>
          <div v-if="selectedDrone.area_radius_m" class="infoRow"><span class="infoHeading">Radius:</span><span class="infoData">{{ selectedDrone.area_radius_m }} m</span></div>
          <div v-if="selectedDrone.classification_region" class="infoRow"><span class="infoHeading">Region:</span><span class="infoData">{{ selectedDrone.classification_region }}</span></div>
        </div>
        <div class="infoBlockSection" style="border-top:1px solid var(--BGCOLOR2);">
          <button @click="goToDetail" class="sidebarButton w-full mb-1" style="font-size:11px;">View Full Details</button>
          <button @click="exportDroneData" class="sidebarButton w-full" style="font-size:11px;">Export CSV</button>
        </div>
      </div>

      <!-- 底部 credits -->
      <div id="credits">Remote ID Monitor</div>

      <!-- 轨迹回放控制条 -->
      <div
        v-if="selectedDrone"
        id="replayBar"
      >
        <span class="font-bold" style="color: var(--TXTCOLOR1);">{{ selectedDrone.uas_id || shortenMac(selectedDrone.mac, true) }}</span>
        <input type="range" v-model.number="timelinePosition" min="0" max="100" class="w-32" />
        <button @click="togglePlayback" class="sidebarButton" style="font-size:11px; padding:1px 6px;">{{ isPlaying ? '⏸' : '▶' }}</button>
        <button @click="stopPlayback" class="sidebarButton" style="font-size:11px; padding:1px 6px;">■</button>
        <button @click="selectedDrone = null; closeInfoBlock()" class="text-xs" style="color: var(--TXTCOLOR2);">✕</button>
      </div>

      <!-- 警报面板 (地图左下) -->
      <div
        v-if="showAlertsPanel && store.alerts.filter(a => a).length > 0"
        id="alerts_panel"
      >
        <div class="flex items-center justify-between px-2 py-1 text-xs font-bold text-white" style="background-color: var(--ACCENT);">
          <span>Alerts ({{ store.alerts.filter(a => a).length }})</span>
          <button @click="showAlertsPanel = false" class="text-white text-xs">✕</button>
        </div>
        <div
          v-for="(alert, idx) in store.alerts.filter(a => a)"
          :key="idx"
          class="px-2 py-1 text-xs"
          style="border-bottom: 1px solid var(--BGCOLOR2);"
        >
          <div class="font-bold" :style="{ color: getAlertColor(alert.type) }">{{ alert.type }}</div>
          <div style="color: var(--TXTCOLOR2);">{{ alert.message }}</div>
          <div class="opacity-50" style="font-size:10px;">{{ alert.timestamp }}</div>
        </div>
      </div>

      <button
        v-if="!showAlertsPanel && store.alerts.filter(a => a).length > 0"
        @click="showAlertsPanel = true"
        class="absolute bottom-2 left-2 z-[200] sidebarButton text-xs px-2"
        :style="{ backgroundColor: '#e53e3e' }"
      >{{ store.alerts.filter(a => a).length }} Alerts</button>
    </div>

    <!-- ========== 拖拽手柄 ========== -->
    <div
      id="splitter"
      v-show="sidebarOpen && mapOpen"
      @mousedown="startResize"
    ></div>

    <!-- ========== 右侧边栏 ========== -->
    <div
      id="sidebar_container"
      v-show="sidebarOpen"
      :style="{ width: sidebarWidth + 'px' }"
    >
      <!-- 侧边栏头部 -->
      <div id="sidebar_header">
        <div class="flex items-center gap-2">
          <span class="status-dot live"></span>
          <span id="header_title">Remote ID</span>
        </div>
        <div class="flex items-center gap-1">
          <span id="header_count">{{ store.activeDrones.length }} drones</span>
          <button @click="sidebarOpen = false" class="text-white opacity-60 hover:opacity-100" title="Hide sidebar" style="font-size:14px; line-height:1;">✕</button>
        </div>
      </div>

      <!-- 搜索栏 -->
      <div id="search_bar">
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Filter..."
          id="search_input"
        />
      </div>

      <!-- 过滤 + 列选择栏（侧边栏内） -->
      <div id="sidebar_controls">
        <div class="flex items-center justify-between">
          <span class="controls-label">Filters / Columns</span>
          <button
            @click="showColumnPanel = !showColumnPanel"
            class="text-xs"
            style="color: var(--ACCENT); cursor: pointer;"
          >{{ showColumnPanel ? '▲ hide' : '▼ show' }}</button>
        </div>

        <!-- 展开的列选择 + 过滤器面板 -->
        <div v-if="showColumnPanel" id="sidebar_column_panel">
          <!-- 过滤器行 -->
          <div class="filter-row">
            <span class="filter-section-title">Filters:</span>
            <label class="filter-chip" :class="{ active: showCompliant }">
              <input type="checkbox" v-model="showCompliant" class="sr-only" />
              Compliant
            </label>
            <label class="filter-chip" :class="{ active: showNonCompliant }">
              <input type="checkbox" v-model="showNonCompliant" class="sr-only" />
              Non-compliant
            </label>
            <label class="filter-chip" :class="{ active: showStale }">
              <input type="checkbox" v-model="showStale" class="sr-only" />
              Stale
            </label>
          </div>

          <!-- 列选择 -->
          <div class="columns-grid">
            <label
              v-for="col in availableColumns"
              :key="col.key"
              class="column-chip"
              :class="{ active: visibleColumns[col.key] }"
            >
              <input
                type="checkbox"
                :checked="visibleColumns[col.key]"
                @change="toggleColumn(col.key)"
                class="sr-only"
              />
              {{ col.label }}
            </label>
          </div>
        </div>
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
                :class="['text-right', col.key === 'ua_type' ? '' : '']"
              >{{ col.label }}</th>
              <th class="text-right">Seen</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="drone in filteredDrones"
              :key="drone.mac"
              class="plane_table_row"
              :class="getRowClass(drone)"
              @click="selectDrone(drone)"
              :style="selectedDrone?.mac === drone.mac ? 'background-color: var(--ACCENT); color: #fff;' : ''"
            >
              <td class="icaoCodeColumn">
                <div class="font-bold">{{ drone.uas_id || shortenMac(drone.mac, false) }}</div>
                <div class="opacity-50" style="font-size:9px;">{{ drone.mac }}</div>
              </td>
              <td
                v-for="col in visibleColumnList"
                :key="col.key"
                :class="col.align === 'right' ? 'text-right font-mono' : ''"
                :style="{ fontSize: col.size || 'var(--FS2)' }"
              >
                <template v-if="col.key === 'ua_type'">{{ drone.ua_type || '?' }}</template>
                <template v-else-if="col.key === 'standard'">{{ drone.standard || '-' }}</template>
                <template v-else-if="col.key === 'altitude'">{{ drone.altitude != null ? Math.round(drone.altitude) + 'm' : '-' }}</template>
                <template v-else-if="col.key === 'latitude'">{{ drone.latitude != null ? drone.latitude.toFixed(4) : '-' }}</template>
                <template v-else-if="col.key === 'longitude'">{{ drone.longitude != null ? drone.longitude.toFixed(4) : '-' }}</template>
                <template v-else-if="col.key === 'speed'">{{ drone.speed != null ? drone.speed.toFixed(1) + 'm/s' : '-' }}</template>
                <template v-else-if="col.key === 'heading'">{{ drone.heading != null ? drone.heading + '°' : '-' }}</template>
                <template v-else-if="col.key === 'status'">{{ drone.status || '-' }}</template>
                <template v-else-if="col.key === 'id_type'">{{ drone.id_type || '-' }}</template>
                <template v-else-if="col.key === 'signal'">{{ drone.signal_strength || '-' }}</template>
                <template v-else-if="col.key === 'china_compliant'">
                  <span :style="{ color: drone.china_compliant ? '#38a169' : '#e53e3e' }">{{ drone.china_compliant ? '✓' : '✗' }}</span>
                </template>
                <template v-else-if="col.key === 'first_seen'">{{ formatTimeShort(drone.first_seen) }}</template>
                <template v-else-if="col.key === 'operator_lat'">{{ drone.operator_latitude != null ? drone.operator_latitude.toFixed(4) : '-' }}</template>
                <template v-else-if="col.key === 'operator_lng'">{{ drone.operator_longitude != null ? drone.operator_longitude.toFixed(4) : '-' }}</template>
                <template v-else-if="col.key === 'area_radius'">{{ drone.area_radius_m != null ? drone.area_radius_m + 'm' : '-' }}</template>
                <template v-else-if="col.key === 'region'">{{ drone.classification_region || '-' }}</template>
                <template v-else>-</template>
              </td>
              <td class="text-right font-mono" style="font-size: var(--FS2);">{{ timeAgoShort(drone.last_seen) }}</td>
            </tr>
            <tr v-if="filteredDrones.length === 0">
              <td :colspan="2 + visibleColumnList.length" class="text-center py-4" style="color: var(--BGCOLOR2);">No drones detected</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 侧边栏底部 -->
      <div id="sidebar_footer">
        <div class="flex gap-1">
          <button @click="exportData" class="sidebarButton" title="Export CSV">CSV</button>
          <button @click="showAllTrajectories" class="sidebarButton" title="Show all traces">Trk</button>
          <button @click="clearTrajectories" class="sidebarButton" title="Clear traces">Clr</button>
        </div>
        <span class="text-xs" style="color: var(--TXTCOLOR2);">{{ store.alerts.filter(a => a).length }} alerts</span>
      </div>
    </div>

    <!-- ========== 隐藏状态下的恢复按钮 ========== -->
    <!-- 地图隐藏时：显示恢复地图按钮 -->
    <button
      v-if="!mapOpen"
      @click="mapOpen = true"
      class="absolute z-[9999] top-2 left-2 sidebarButton text-xs px-3 py-1.5"
      title="Show map"
    >🗺 Map</button>

    <!-- 侧边栏隐藏时：显示恢复侧边栏按钮 -->
    <button
      v-if="!sidebarOpen"
      @click="sidebarOpen = true"
      class="absolute z-[9999] top-2 right-2 sidebarButton text-xs px-3 py-1.5"
      title="Show sidebar"
    >☰ Sidebar</button>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import * as L from 'leaflet'
import 'leaflet/dist/leaflet.css'

import { timeAgo, isRecent, shortenMac, formatCoord, formatTime, downloadBlob } from '@/utils/helpers'
import { setupLeafletIcons, createDroneIcon } from '@/utils/leaflet-setup'
import {
  fetchDroneDetail, fetchDroneTrajectory, fetchDroneExport,
  fetchAlerts, dronesToCSV, connectWebSocket
} from '@/utils/api'
import { useDroneStore } from '@/stores/drones'
import logger from '@/utils/logger'

setupLeafletIcons()

const router = useRouter()
const store = useDroneStore()

// ---- 地图类型 ----
const TDT_TOKEN = '66ad6f2f6216ab57401ed9907e94cd43'
const mapTypeOptions = [
  { key: 'osm', label: 'OSM' },
  { key: 'tianditu', label: '天地图' },
  { key: 'tiandituImg', label: '影像' }
]

const mapTypes = {
  osm: {
    url: 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png',
    attribution: '&copy; OpenStreetMap'
  },
  tianditu: {
    url: `https://t{s}.tianditu.gov.cn/vec_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=vec&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILECOL={x}&TILEROW={y}&TILEMATRIX={z}&tk=${TDT_TOKEN}`,
    subdomains: ['0','1','2','3','4','5','6','7'],
    annotationUrl: `https://t{s}.tianditu.gov.cn/cva_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=cva&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILECOL={x}&TILEROW={y}&TILEMATRIX={z}&tk=${TDT_TOKEN}`,
    attribution: '&copy; 天地图'
  },
  tiandituImg: {
    url: `https://t{s}.tianditu.gov.cn/img_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=img&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILECOL={x}&TILEROW={y}&TILEMATRIX={z}&tk=${TDT_TOKEN}`,
    subdomains: ['0','1','2','3','4','5','6','7'],
    annotationUrl: `https://t{s}.tianditu.gov.cn/cva_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=cva&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILECOL={x}&TILEROW={y}&TILEMATRIX={z}&tk=${TDT_TOKEN}`,
    attribution: '&copy; 天地图'
  }
}

// ---- 状态 ----
const mapContainer = ref(null)
const map = ref(null)
const droneMarkers = ref({})
const trajectoryLayers = ref({})
const playbackMarkers = ref({})

const sidebarOpen = ref(true)
const mapOpen = ref(true)
const sidebarWidth = ref(350)
const showInfoBlock = ref(false)
const selectedDrone = ref(null)
const selectedDroneDetail = ref(null)
const selectedDroneTrajectory = ref([])
const currentMapType = ref('osm')

const timelinePosition = ref(0)
const isPlaying = ref(false)

const searchQuery = ref('')
const showSettings = ref(false)
const showCompliant = ref(true)
const showNonCompliant = ref(true)
const showStale = ref(true)
const showAlertsPanel = ref(false)
const showColumnPanel = ref(true)

// ---- 列选择配置 ----
const availableColumns = [
  { key: 'ua_type', label: 'Type', align: 'left', size: 'var(--FS2)', default: true },
  { key: 'standard', label: 'Std', align: 'left', size: 'var(--FS2)', default: true },
  { key: 'altitude', label: 'Alt', align: 'right', size: 'var(--FS2)', default: true },
  { key: 'latitude', label: 'Lat', align: 'right', size: 'var(--FS2)', default: true },
  { key: 'longitude', label: 'Lon', align: 'right', size: 'var(--FS2)', default: true },
  { key: 'speed', label: 'Speed', align: 'right', size: 'var(--FS2)', default: false },
  { key: 'heading', label: 'Hdg', align: 'right', size: 'var(--FS2)', default: false },
  { key: 'status', label: 'Status', align: 'left', size: 'var(--FS2)', default: false },
  { key: 'id_type', label: 'ID Type', align: 'left', size: '10px', default: false },
  { key: 'signal', label: 'Signal', align: 'left', size: 'var(--FS2)', default: false },
  { key: 'china_compliant', label: 'Compl', align: 'left', size: 'var(--FS2)', default: false },
  { key: 'first_seen', label: 'First', align: 'right', size: '10px', default: false },
  { key: 'operator_lat', label: 'Op Lat', align: 'right', size: '10px', default: false },
  { key: 'operator_lng', label: 'Op Lng', align: 'right', size: '10px', default: false },
  { key: 'area_radius', label: 'Radius', align: 'right', size: '10px', default: false },
  { key: 'region', label: 'Region', align: 'left', size: '10px', default: false }
]

const visibleColumns = ref(
  Object.fromEntries(availableColumns.map(c => [c.key, c.default]))
)

const visibleColumnList = computed(() =>
  availableColumns.filter(c => visibleColumns.value[c.key])
)

const toggleColumn = (key) => {
  visibleColumns.value[key] = !visibleColumns.value[key]
}

let ws = null
let pollInterval = null
let playbackInterval = null

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

  if (!showStale.value) {
    drones = drones.filter(d => isRecent(d.last_seen, 30))
  }
  if (!showCompliant.value) {
    drones = drones.filter(d => !d.china_compliant)
  }
  if (!showNonCompliant.value) {
    drones = drones.filter(d => d.china_compliant !== false)
  }

  return drones
})

// ---- 表格行颜色 (tar1090 tableColors 风格) ----
const getRowClass = (drone) => {
  if (!drone) return ''
  const classes = []
  if (!isRecent(drone.last_seen, 5)) classes.push('stale_row')
  if (drone.china_compliant === false) classes.push('noncompliant_row')
  else if (drone.standard === 'ASTM') classes.push('astm_row')
  return classes.join(' ')
}

const getAlertColor = (type) => {
  if (!type) return 'var(--TXTCOLOR2)'
  const t = type.toLowerCase()
  if (t.includes('non-compliant') || t.includes('violation')) return '#e53e3e'
  if (t.includes('warn')) return '#dd6b20'
  return '#3182ce'
}

// ---- 时间显示 ----
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

// ---- 侧边栏拖拽（右侧边栏：宽度 = 窗口宽 - 鼠标X） ----
let isResizing = false
const startResize = (e) => {
  isResizing = true
  document.addEventListener('mousemove', doResize)
  document.addEventListener('mouseup', stopResize)
  e.preventDefault()
}
const doResize = (e) => {
  if (!isResizing) return
  const w = Math.max(250, Math.min(600, window.innerWidth - e.clientX))
  sidebarWidth.value = w
}
const stopResize = () => {
  isResizing = false
  document.removeEventListener('mousemove', doResize)
  document.removeEventListener('mouseup', stopResize)
}

// ---- 详情面板 ----
const closeInfoBlock = () => {
  showInfoBlock.value = false
  selectedDrone.value = null
  selectedDroneDetail.value = null
}

// ---- 地图初始化 ----
const initMap = () => {
  if (!mapContainer.value) {
    logger.warn('initMap: mapContainer ref is null')
    return
  }
  const rect = mapContainer.value.getBoundingClientRect()
  if (rect.width === 0 || rect.height === 0) {
    logger.warn('initMap: mapContainer has zero size, retrying...', rect)
    setTimeout(initMap, 100)
    return
  }
  if (map.value) map.value.remove()

  map.value = L.map(mapContainer.value, {
    zoomControl: false,
    attributionControl: true,
  }).setView([23.14287, 113.26026], 10)

  L.control.zoom({ position: 'bottomright' }).addTo(map.value)
  updateMapLayer()

  setTimeout(() => {
    if (map.value) map.value.invalidateSize()
  }, 300)
}

const updateMapLayer = () => {
  if (!map.value) return
  map.value.eachLayer(layer => {
    if (layer instanceof L.TileLayer) map.value.removeLayer(layer)
  })
  const config = mapTypes[currentMapType.value]
  if (currentMapType.value === 'osm') {
    L.tileLayer(config.url, { attribution: config.attribution }).addTo(map.value)
  } else {
    L.tileLayer(config.url, { subdomains: config.subdomains, attribution: config.attribution }).addTo(map.value)
    L.tileLayer(config.annotationUrl, { subdomains: config.subdomains, attribution: config.attribution }).addTo(map.value)
  }
}

const changeMapType = (key) => {
  currentMapType.value = key
  updateMapLayer()
}

// ---- 无人机标记 ----
const getDroneColor = (drone) => {
  if (!drone.china_compliant && drone.china_compliant !== undefined) return 'red'
  if (drone.standard === 'ASTM') return 'blue'
  return 'green'
}

const createPopupContent = (drone, mac) => `
  <div style="font-size:13px;">
    <b>${drone.uas_id || 'Unknown'}</b><br>
    MAC: ${mac}<br>
    Type: ${drone.ua_type || 'N/A'}<br>
    Alt: ${drone.altitude ? drone.altitude.toFixed(1) : 'N/A'}m<br>
    <button class="view-details-btn" style="margin-top:4px; padding:2px 6px; background: var(--ACCENT); color:#fff; border:none; border-radius:2px; cursor:pointer; font-size:11px;">
      Details
    </button>
  </div>
`

const updateDroneMarkers = (drones) => {
  if (!map.value) return
  const activeMacs = new Set(drones.map(d => d.mac))
  Object.keys(droneMarkers.value).forEach(mac => {
    if (!activeMacs.has(mac)) {
      map.value.removeLayer(droneMarkers.value[mac])
      delete droneMarkers.value[mac]
    }
  })
  drones.forEach(drone => {
    if (!drone.latitude || !drone.longitude) return
    const color = getDroneColor(drone)
    if (droneMarkers.value[drone.mac]) {
      const marker = droneMarkers.value[drone.mac]
      marker.setLatLng([drone.latitude, drone.longitude])
      marker.setIcon(createDroneIcon(color, 12))
      marker.setPopupContent(createPopupContent(drone, drone.mac))
    } else {
      const marker = L.marker([drone.latitude, drone.longitude], {
        icon: createDroneIcon(color, 12)
      }).addTo(map.value)
      marker.bindPopup(createPopupContent(drone, drone.mac))
      droneMarkers.value[drone.mac] = marker
    }
  })
}

const updateDroneMarker = (mac, droneData) => {
  if (!map.value || !droneMarkers.value[mac]) return
  const marker = droneMarkers.value[mac]
  marker.setLatLng([droneData.latitude, droneData.longitude])
  marker.setPopupContent(createPopupContent(droneData, mac))
}

// ---- 无人机选择 ----
const selectDrone = async (drone) => {
  selectedDrone.value = drone
  showInfoBlock.value = true

  if (map.value && drone.latitude && drone.longitude) {
    map.value.setView([drone.latitude, drone.longitude], 14)
    droneMarkers.value[drone.mac]?.openPopup()
  }

  try {
    const detail = await fetchDroneDetail(drone.mac)
    selectedDroneDetail.value = detail
  } catch (e) {
    logger.error('Fetch drone detail error:', e)
  }
}

const goToDetail = () => {
  if (selectedDrone.value) {
    router.push(`/drone/${selectedDrone.value.mac}`)
  }
}

// ---- 地图 Popup 点击处理 ----
const handleMapClick = (e) => {
  const el = e.target
  if (el.classList.contains('view-details-btn')) {
    const popup = el.closest('.leaflet-popup-content')
    if (popup) {
      const nameEl = popup.querySelector('b')
      map.value?.closePopup()
      const name = nameEl?.textContent
      const drone = store.activeDrones.find(d => d.uas_id === name || d.mac === name)
      if (drone) selectDrone(drone)
    }
  }
}

// ---- 轨迹 ----
const showAllTrajectories = async () => {
  if (!map.value) return
  clearTrajectories()
  for (const drone of store.activeDrones) {
    await showDroneTrajectory(drone.mac)
  }
}

const showDroneTrajectory = async (mac) => {
  if (!map.value) return
  try {
    const points = await fetchDroneTrajectory(mac)
    if (points.length > 1) {
      const latLngs = points.map(p => [p.latitude, p.longitude])
      const polyline = L.polyline(latLngs, {
        color: 'var(--ACCENT)', weight: 2, opacity: 0.6
      }).addTo(map.value)
      trajectoryLayers.value[mac] = polyline
    }
  } catch (e) {
    logger.error('Trajectory error:', e)
  }
}

const clearTrajectories = () => {
  if (!map.value) return
  Object.values(trajectoryLayers.value).forEach(layer => {
    map.value.hasLayer(layer) && map.value.removeLayer(layer)
  })
  trajectoryLayers.value = {}
  Object.values(playbackMarkers.value).forEach(m => {
    map.value.hasLayer(m) && map.value.removeLayer(m)
  })
  playbackMarkers.value = {}
}

// ---- 播放 ----
const loadDroneTrajectory = async (mac) => {
  try {
    selectedDroneTrajectory.value = await fetchDroneTrajectory(mac)
  } catch (e) {
    logger.error('Load trajectory error:', e)
  }
}

const togglePlayback = async () => {
  if (!selectedDrone.value) return
  if (isPlaying.value) { stopPlayback(); return }
  if (!selectedDroneTrajectory.value.length) {
    await loadDroneTrajectory(selectedDrone.value.mac)
  }
  if (!selectedDroneTrajectory.value.length) return
  isPlaying.value = true
  startPlayback()
}

const stopPlayback = () => {
  isPlaying.value = false
  if (playbackInterval) { clearInterval(playbackInterval); playbackInterval = null }
}

const startPlayback = () => {
  if (!map.value || !selectedDrone.value) return
  const points = selectedDroneTrajectory.value
  let idx = Math.floor((timelinePosition.value / 100) * points.length)
  playbackInterval = setInterval(() => {
    if (!isPlaying.value || !map.value) { stopPlayback(); return }
    if (idx < points.length) {
      const p = points[idx]
      if (playbackMarkers.value[selectedDrone.value.mac]) {
        map.value.removeLayer(playbackMarkers.value[selectedDrone.value.mac])
      }
      const marker = L.circleMarker([p.latitude, p.longitude], {
        radius: 6, color: 'var(--ACCENT)', fillColor: '#fff', fillOpacity: 1, weight: 2
      }).addTo(map.value)
      playbackMarkers.value[selectedDrone.value.mac] = marker
      timelinePosition.value = (idx / points.length) * 100
      idx++
    } else { stopPlayback() }
  }, 150)
}

// ---- 导出 ----
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
  if (!selectedDrone.value) return
  try {
    const data = await fetchDroneExport(selectedDrone.value.mac)
    const csv = dronesToCSV([data])
    downloadBlob(new Blob([csv], { type: 'text/csv;charset=utf-8;' }),
      `drone_${selectedDrone.value.mac.replace(/:/g,'_')}_${new Date().toISOString().split('T')[0]}.csv`)
  } catch (e) { logger.error('Export error:', e) }
}

// ---- WebSocket ----
const initWebSocket = () => {
  try {
    ws = connectWebSocket({
      onDroneUpdate: (mac, data) => updateDroneMarker(mac, data)
    })
  } catch (e) { logger.error('WS error:', e) }
}

// ---- 数据轮询 ----
const refreshData = async () => {
  try {
    const drones = await store.loadActiveDrones()
    store.cleanStaleDrones()
    updateDroneMarkers(drones)
    await store.loadAlerts()
  } catch (e) { logger.error('Refresh error:', e) }
}

// ---- 生命周期 ----
onMounted(() => {
  nextTick(() => {
    requestAnimationFrame(() => {
      initMap()
      initWebSocket()
      if (mapContainer.value) {
        mapContainer.value.addEventListener('click', handleMapClick)
      }
    })
  })
  refreshData()
  pollInterval = setInterval(refreshData, 5000)
})

onUnmounted(() => {
  if (playbackInterval) clearInterval(playbackInterval)
  if (pollInterval) clearInterval(pollInterval)
  if (ws) { ws.close(); ws = null }
  if (map.value) {
    map.value.closePopup()
    map.value.eachLayer(l => map.value.removeLayer(l))
    map.value.remove()
    map.value = null
  }
  document.removeEventListener('mousemove', doResize)
  document.removeEventListener('mouseup', stopResize)
})
</script>

<style>
/* =========================================================================
   tar1090-style layout CSS — all non-scoped for map container sizing
   ========================================================================= */

#layout_container {
  display: flex;
  height: 100vh;
  overflow: hidden;
}

/* ---- 右侧边栏 ---- */
#sidebar_container {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  height: 100%;
  overflow: hidden;
  border-left: 1px solid var(--BGCOLOR2);
  background: var(--BGCOLOR1);
}

#sidebar_header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 8px;
  flex-shrink: 0;
  background-color: var(--ACCENT);
  color: #FFF;
}

#header_title {
  color: #FFF;
  font-weight: bold;
  font-size: var(--FS2);
}

#header_count {
  font-size: var(--FS1);
  opacity: 0.8;
}

/* 搜索栏 */
#search_bar {
  padding: 4px 6px;
  flex-shrink: 0;
  background: var(--BGCOLOR1);
  border-bottom: 1px solid var(--BGCOLOR2);
}

#search_input {
  width: 100%;
  height: 22px;
  padding: 0 4px;
  font-size: var(--FS1);
  background: #fff;
  border: 1px solid var(--BGCOLOR2);
  outline: none;
}

/* 侧边栏内 过滤+列选择栏 */
#sidebar_controls {
  flex-shrink: 0;
  padding: 4px 6px;
  background: var(--BGCOLOR1);
  border-bottom: 1px solid var(--BGCOLOR2);
}

.controls-label {
  font-size: var(--FS1);
  color: var(--TXTCOLOR2);
  font-weight: 600;
}

#sidebar_column_panel {
  margin-top: 4px;
}

.filter-row {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
  margin-bottom: 6px;
}

.filter-section-title {
  font-size: 10px;
  color: var(--TXTCOLOR2);
  margin-right: 2px;
}

.filter-chip,
.column-chip {
  display: inline-flex;
  align-items: center;
  padding: 1px 6px;
  font-size: 10px;
  border-radius: 3px;
  border: 1px solid var(--BGCOLOR2);
  background: #fff;
  cursor: pointer;
  user-select: none;
  white-space: nowrap;
  transition: all 0.1s;
}

.filter-chip:hover,
.column-chip:hover {
  border-color: var(--ACCENT);
}

.filter-chip.active,
.column-chip.active {
  background: var(--ACCENT);
  color: #fff;
  border-color: var(--ACCENT);
}

.columns-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 3px;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0,0,0,0);
  border: 0;
}

/* 侧边栏表格区域 */
#sidebar_canvas {
  flex: 1 1 auto;
  overflow-y: auto;
  overflow-x: auto;
}

#planesTable {
  width: 100%;
  font-size: var(--FS2);
  white-space: nowrap;
  border-collapse: collapse;
}

.aircraft_table_header th {
  position: sticky;
  top: 0;
  z-index: 10;
  background-color: var(--ACCENT);
  color: #FFF;
  font-weight: normal;
  padding: 4px 4px;
  box-shadow: 0 2px 2px -1px rgba(0,0,0,.5);
}

#planesTable td {
  padding: 2px 4px;
  cursor: default;
}

.icaoCodeColumn {
  font-family: monospace;
  text-transform: uppercase;
}

/* 表格行颜色 */
.plane_table_row.stale_row {
  opacity: 0.45;
}

.plane_table_row.noncompliant_row {
  background-color: #ffe0e0;
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
  border-top: 1px solid var(--BGCOLOR2);
  background: var(--BGCOLOR1);
}

/* ---- 拖拽手柄 ---- */
#splitter {
  flex-shrink: 0;
  width: 4px;
  background: var(--BGCOLOR2);
  cursor: ew-resize;
  z-index: 50;
}

/* ---- 地图区域 ---- */
#map_container {
  flex: 1 1 auto;
  position: relative;
  height: 100%;
  min-height: 0;
}

#map_canvas {
  position: absolute;
  inset: 0;
}

#map_canvas .leaflet-container {
  height: 100% !important;
  width: 100% !important;
}

/* ---- 左上角按钮组 ---- */
#header_side {
  position: absolute;
  top: 6px;
  left: 6px;
  z-index: 999;
  display: flex;
  align-items: center;
}

.toggle_sidebar_button {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 38px;
  border-radius: 2px;
  background: rgba(255,255,255,.85);
  border: 1px solid var(--BGCOLOR2);
  cursor: pointer;
  color: var(--TXTCOLOR2);
}

/* ---- 设置面板 (tar1090 #settings_infoblock) ---- */
#settings_infoblock {
  position: absolute;
  top: 1%;
  left: 1%;
  z-index: 9999;
  max-height: 90%;
  overflow-y: auto;
  background: var(--BGCOLOR1);
  border: 1px solid var(--BGCOLOR2);
  border-radius: 2px;
  box-shadow: 0 4px 16px rgba(0,0,0,.2);
  min-width: 320px;
  max-width: 500px;
}

#settings_header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  background-color: var(--ACCENT);
  color: #FFF;
}

#settings_content {
  display: flex;
  gap: 16px;
  padding: 10px;
}

.settingsColumn {
  flex: 1;
  min-width: 140px;
}

.settingsSectionTitle {
  font-weight: bold;
  font-size: var(--FS2);
  color: var(--TXTCOLOR1);
  margin-bottom: 4px;
  border-bottom: 1px solid var(--BGCOLOR2);
  padding-bottom: 2px;
}

.settingsCheckboxLabel {
  display: flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
  font-size: var(--FS1);
  padding: 1px 0;
  white-space: nowrap;
}

.settingsCheckboxLabel input[type="checkbox"] {
  width: 12px;
  height: 12px;
  margin: 0;
}

/* ---- 选中信息块 (tar1090 #selected_infoblock) ---- */
#selected_infoblock {
  position: absolute;
  bottom: 36px;
  left: 6px;
  z-index: 999;
  max-height: 55vh;
  overflow-y: auto;
  background: var(--BGCOLOR1);
  border: 1px solid var(--BGCOLOR2);
  border-radius: 2px;
  box-shadow: 0 2px 8px rgba(0,0,0,.15);
  width: 200px;
  font-size: var(--FS2);
}

#infoblock_close {
  position: absolute;
  top: 2px;
  right: 4px;
  cursor: pointer;
  font-size: 16px;
  line-height: 1;
  color: var(--TXTCOLOR2);
  z-index: 1;
}

#infoblock_close:hover {
  color: #e53e3e;
}

/* ---- credits ---- */
#credits {
  position: absolute;
  bottom: 4px;
  left: 50%;
  transform: translateX(-50%);
  opacity: 0.7;
  z-index: 99;
  font-size: var(--FS1);
  color: var(--TXTCOLOR1);
  pointer-events: none;
}

/* ---- 回放条 ---- */
#replayBar {
  position: absolute;
  bottom: 28px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 100;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  border-radius: 2px;
  box-shadow: 0 2px 6px rgba(0,0,0,.15);
  background: var(--BGCOLOR1);
  border: 1px solid var(--BGCOLOR2);
  font-size: var(--FS1);
}

/* ---- 警报面板 ---- */
#alerts_panel {
  position: absolute;
  bottom: 6px;
  left: 6px;
  z-index: 200;
  border-radius: 2px;
  box-shadow: 0 2px 8px rgba(0,0,0,.15);
  overflow: hidden;
  background: var(--BGCOLOR1);
  border: 1px solid var(--BGCOLOR2);
  max-height: 220px;
  overflow-y: auto;
  width: 280px;
}

/* ---- infoBlockSection row clearfix ---- */
.infoRow {
  clear: both;
}

.infoHeading {
  display: inline;
  margin-right: 4px;
}

.infoData {
  float: right;
  text-align: right;
}

.infoBlockSection {
  padding: 6px 8px;
}
</style>
