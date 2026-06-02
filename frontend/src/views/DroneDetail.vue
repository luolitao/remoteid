<!-- src/views/DroneDetail.vue — tar1090 infoBlock style -->
<template>
  <div class="h-screen flex flex-col" style="background: var(--BGCOLOR1); font-size: var(--FS2);">

    <!-- 顶部栏 (tar1090 风格) -->
    <header class="flex items-center justify-between px-3 py-2 flex-shrink-0" style="background: var(--BGCOLOR1); border-bottom: 1px solid var(--BGCOLOR2);">
      <div class="flex items-center gap-3">
        <RouterLink to="/" class="text-sm hover:opacity-70" style="color: var(--ACCENT);">
          ← Back
        </RouterLink>
        <div>
          <span class="identLarge">{{ drone?.uas_id || 'Unknown' }}</span>
          <span class="text-xs ml-2 opacity-60" style="color: var(--TXTCOLOR2);">
            {{ drone?.mac }}
          </span>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <span class="px-2 py-0.5 rounded text-xs font-bold" style="background: #dcfce7; color: #166534;">
          {{ drone?.standard || 'ASTM F3411-22a' }}
        </span>
        <button @click="exportData" class="sidebarButton text-xs px-2">CSV</button>
        <button @click="refreshData" class="sidebarButton text-xs px-2">↻</button>
      </div>
    </header>

    <!-- 主内容区 -->
    <div class="flex-1 flex overflow-hidden">
      <!-- 左侧: 地图 + 位置历史 -->
      <div class="flex-1 flex flex-col overflow-hidden">
        <!-- 地图 -->
        <div class="flex-1 relative">
          <div ref="mapContainer" class="absolute inset-0"></div>
        </div>
        <!-- 位置历史表 (tar1090 表格风格) -->
        <div v-if="positionHistory.length > 0" class="flex-shrink-0" style="max-height: 200px; overflow-y: auto; border-top: 1px solid var(--BGCOLOR2);">
          <table class="w-full text-xs" style="font-size: var(--FS2);">
            <thead>
              <tr class="aircraft_table_header sticky top-0 z-10" style="background: var(--ACCENT); color: #FFF;">
                <th class="text-left px-2 py-1 font-normal">Time</th>
                <th class="text-right px-2 py-1 font-normal">Lat</th>
                <th class="text-right px-2 py-1 font-normal">Lon</th>
                <th class="text-right px-2 py-1 font-normal">Alt</th>
                <th class="text-right px-2 py-1 font-normal">Spd</th>
                <th class="text-right px-2 py-1 font-normal">Hdg</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(pos, i) in positionHistory" :key="i" class="border-b" style="border-color: var(--BGCOLOR2);">
                <td class="px-2 py-1">{{ formatTime(pos.timestamp) }}</td>
                <td class="px-2 py-1 text-right font-mono">{{ formatCoord(pos.latitude) }}</td>
                <td class="px-2 py-1 text-right font-mono">{{ formatCoord(pos.longitude) }}</td>
                <td class="px-2 py-1 text-right font-mono">{{ pos.altitude ? pos.altitude.toFixed(1)+'m' : '-' }}</td>
                <td class="px-2 py-1 text-right font-mono">{{ pos.speed ? pos.speed.toFixed(1) : '-' }}</td>
                <td class="px-2 py-1 text-right font-mono">{{ pos.heading ? pos.heading.toFixed(0)+'°' : '-' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- 右侧: 详情面板 (tar1090 #selected_infoblock 风格) -->
      <div class="flex-shrink-0 overflow-y-auto border-l" style="width: 240px; background: var(--BGCOLOR1); border-color: var(--BGCOLOR2);">

        <!-- 系统信息 -->
        <div class="infoBlockSection" style="border-bottom: 1px solid var(--BGCOLOR2);">
          <div style="color: var(--TXTCOLOR1); font-weight: bold; margin-bottom: 4px;">System Info</div>
          <div>
            <div class="infoHeading">Standard:</div>
            <div class="infoData" :style="{ color: drone?.standard ? '#38a169' : '#999' }">{{ drone?.standard || 'N/A' }}</div>
          </div>
          <div v-if="drone?.source">
            <div class="infoHeading">Source:</div>
            <div class="infoData">{{ drone.source }}</div>
          </div>
          <div>
            <div class="infoHeading">UA Type:</div>
            <div class="infoData">{{ drone?.ua_type || 'N/A' }}</div>
          </div>
          <div>
            <div class="infoHeading">ID Type:</div>
            <div class="infoData">{{ drone?.id_type || 'N/A' }}</div>
          </div>
          <div v-if="drone?.battery_level">
            <div class="infoHeading">Battery:</div>
            <div class="infoData">{{ drone.battery_level }}</div>
          </div>
          <div v-if="drone?.flight_time">
            <div class="infoHeading">Flight Time:</div>
            <div class="infoData">{{ drone.flight_time }}</div>
          </div>
          <div>
            <div class="infoHeading">Signal:</div>
            <div class="infoData font-mono">{{ drone?.signal_strength || 'N/A' }}</div>
          </div>
          <div>
            <div class="infoHeading">First seen:</div>
            <div class="infoData">{{ formatTime(drone?.first_seen) }}</div>
          </div>
          <div>
            <div class="infoHeading">Last seen:</div>
            <div class="infoData">{{ timeAgo(drone?.last_seen) }}</div>
          </div>
        </div>

        <!-- 位置信息 -->
        <div class="infoBlockSection" style="border-bottom: 1px solid var(--BGCOLOR2);">
          <div style="color: var(--TXTCOLOR1); font-weight: bold; margin-bottom: 4px;">Position</div>
          <div>
            <div class="infoHeading">Latitude:</div>
            <div class="infoData font-mono">{{ formatCoord(drone?.latitude) }}</div>
          </div>
          <div>
            <div class="infoHeading">Longitude:</div>
            <div class="infoData font-mono">{{ formatCoord(drone?.longitude) }}</div>
          </div>
          <div>
            <div class="infoHeading">Altitude:</div>
            <div class="infoData font-mono">{{ drone?.altitude ? drone.altitude.toFixed(1)+' m' : 'N/A' }}</div>
          </div>
          <div>
            <div class="infoHeading">Speed:</div>
            <div class="infoData font-mono">{{ drone?.speed ? drone.speed.toFixed(2)+' m/s' : 'N/A' }}</div>
          </div>
          <div>
            <div class="infoHeading">Heading:</div>
            <div class="infoData font-mono">{{ drone?.heading ? drone.heading.toFixed(1)+'°' : 'N/A' }}</div>
          </div>
        </div>

        <!-- 操作员信息 -->
        <div v-if="drone?.operator_latitude || drone?.operator_id" class="infoBlockSection" style="border-bottom: 1px solid var(--BGCOLOR2);">
          <div style="color: var(--TXTCOLOR1); font-weight: bold; margin-bottom: 4px;">Operator</div>
          <div v-if="drone?.operator_id">
            <div class="infoHeading">Op. ID:</div>
            <div class="infoData font-mono">{{ drone.operator_id }}</div>
          </div>
          <div v-if="drone?.operator_latitude">
            <div class="infoHeading">Position:</div>
            <div class="infoData font-mono text-xs">{{ formatCoord(drone.operator_latitude) }}, {{ formatCoord(drone.operator_longitude) }}</div>
          </div>
          <div v-if="drone.operator_latitude">
            <div class="infoHeading">Op. Altitude:</div>
            <div class="infoData font-mono">{{ drone.operator_altitude ? drone.operator_altitude.toFixed(1)+' m' : 'N/A' }}</div>
          </div>
          <div v-if="drone.area_radius_m">
            <div class="infoHeading">Area Radius:</div>
            <div class="infoData">{{ drone.area_radius_m }} m</div>
          </div>
          <div v-if="drone.classification_region">
            <div class="infoHeading">Region:</div>
            <div class="infoData">{{ drone.classification_region }}</div>
          </div>
        </div>

        <!-- 飞行状态信息 -->
        <div v-if="hasFlightStatus" class="infoBlockSection" style="border-bottom: 1px solid var(--BGCOLOR2);">
          <div style="color: var(--TXTCOLOR1); font-weight: bold; margin-bottom: 4px;">Flight Status</div>
          <div v-if="drone?.flight_status">
            <div class="infoHeading">Status:</div>
            <div class="infoData">{{ drone.flight_status }}</div>
          </div>
          <div v-if="drone?.height_type">
            <div class="infoHeading">Height Type:</div>
            <div class="infoData">{{ drone.height_type }}</div>
          </div>
          <div v-if="drone?.speed_v">
            <div class="infoHeading">Vert Speed:</div>
            <div class="infoData font-mono">{{ drone.speed_v }} m/s</div>
          </div>
          <div v-if="drone?.h_accuracy">
            <div class="infoHeading">H Accuracy:</div>
            <div class="infoData">{{ drone.h_accuracy }}</div>
          </div>
          <div v-if="drone?.v_accuracy">
            <div class="infoHeading">V Accuracy:</div>
            <div class="infoData">{{ drone.v_accuracy }}</div>
          </div>
          <div v-if="drone?.s_accuracy">
            <div class="infoHeading">S Accuracy:</div>
            <div class="infoData">{{ drone.s_accuracy }}</div>
          </div>
          <div v-if="drone?.timestamp">
            <div class="infoHeading">Timestamp:</div>
            <div class="infoData font-mono text-xs">{{ drone.timestamp }}</div>
          </div>
        </div>

        <!-- 警报历史 -->
        <div v-if="alertHistory.length > 0" class="infoBlockSection">
          <div style="color: var(--TXTCOLOR1); font-weight: bold; margin-bottom: 4px;">Alert History</div>
          <div v-for="(alert, i) in alertHistory" :key="i" class="mb-2 text-xs">
            <div class="font-bold" :style="{ color: alert.type?.toLowerCase().includes('non-compliant') ? '#e53e3e' : '#dd6b20' }">
              {{ alert.type }}
            </div>
            <div style="color: var(--TXTCOLOR2);">{{ alert.message }}</div>
            <div class="opacity-50">{{ formatTime(alert.timestamp) }}</div>
          </div>
        </div>

      </div>
    </div>

  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import * as L from 'leaflet'
import 'leaflet/dist/leaflet.css'

import { timeAgo, formatTime, formatCoord, downloadBlob } from '@/utils/helpers'
import { setupLeafletIcons, createDroneIcon } from '@/utils/leaflet-setup'
import { fetchDroneDetail, fetchDroneExport, dronesToCSV } from '@/utils/api'
import logger from '@/utils/logger'

setupLeafletIcons()

const route = useRoute()
const mac = route.params.mac

const mapContainer = ref(null)
let map = null
let droneMarker = null

const drone = ref(null)
const positionHistory = ref([])
const alertHistory = ref([])

// 飞行状态面板是否可见
const hasFlightStatus = computed(() => {
  const d = drone.value
  return d && (d.flight_status || d.height_type || d.speed_v || d.h_accuracy || d.v_accuracy || d.s_accuracy || d.timestamp)
})

// ---- 初始化地图 ----
const initMap = () => {
  if (!mapContainer.value) return
  map = L.map(mapContainer.value, { zoomControl: false }).setView([23.14287, 113.26026], 12)
  L.control.zoom({ position: 'bottomright' }).addTo(map)

  // 高德地图矢量底图（开发模式走 Vite proxy 代理）
  const amapUrl = import.meta.env.DEV
    ? '/amap-vec?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}'
    : 'https://wprd0{s}.is.autonavi.com/appmaptile?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}'
  L.tileLayer(amapUrl, {
    subdomains: import.meta.env.DEV ? [] : ['01','02','03','04'],
    attribution: '&copy; 高德地图'
  }).addTo(map)
}

// ---- 更新标记 ----
const updateMarker = () => {
  if (!map || !drone.value) return
  const d = drone.value
  if (!d.latitude || !d.longitude) return

  if (droneMarker) {
    droneMarker.setLatLng([d.latitude, d.longitude])
  } else {
    droneMarker = L.marker([d.latitude, d.longitude], {
      icon: createDroneIcon('blue', 14)
    }).addTo(map)
    droneMarker.bindPopup(`
      <div style="font-size:13px;">
        <b>${d.uas_id || 'Unknown'}</b><br>
        Alt: ${d.altitude ? d.altitude.toFixed(1) : 'N/A'}m<br>
        Type: ${d.ua_type || 'N/A'}
      </div>
    `)
  }
  map.setView([d.latitude, d.longitude], 15)
}

// ---- 获取数据 ----
const fetchDetails = async () => {
  try {
    const data = await fetchDroneDetail(mac)
    drone.value = data
    positionHistory.value = (data.position_history || []).slice(-20)
    alertHistory.value = data.alert_history || []
    updateMarker()
  } catch (e) { logger.error('Fetch error:', e) }
}

const refreshData = () => fetchDetails()

// ---- 导出 ----
const exportData = async () => {
  try {
    const data = await fetchDroneExport(mac)
    const csv = dronesToCSV([data])
    downloadBlob(new Blob([csv], { type: 'text/csv;charset=utf-8;' }),
      `drone_${mac.replace(/:/g,'_')}_${new Date().toISOString().split('T')[0]}.csv`)
  } catch (e) { logger.error('Export error:', e) }
}

let pollInterval = null

onMounted(() => {
  initMap()
  fetchDetails()
  pollInterval = setInterval(fetchDetails, 5000)
})

onUnmounted(() => {
  if (pollInterval) clearInterval(pollInterval)
  if (map) { map.remove(); map = null }
})
</script>

<style scoped>
::deep(.leaflet-container) {
  height: 100%;
  width: 100%;
}
</style>
