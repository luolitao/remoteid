<!-- src/components/MapArea.vue -->
<template>
  <div id="map_container">
    <div id="map_canvas" ref="mapContainer"></div>
    
    <!-- 右上角按钮组 -->
    <div id="header_side">
      <button @click="$emit('toggle-map')" class="toggle_sidebar_button" title="Hide map">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <rect x="3" y="3" width="18" height="18" rx="2" />
          <line x1="9" y1="3" x2="9" y2="21" />
        </svg>
      </button>
      <div class="flex gap-0.5 ml-2">
        <button
          v-for="mt in mapTypeOptions"
          :key="mt.key"
          @click="changeMapType(mt.key)"
          class="sidebarButton"
          :style="currentMapType === mt.key ? 'background-color: var(--ACCENT); color: #fff;' : ''"
          style="font-size:11px; padding:2px 6px;"
        >{{ mt.label }}</button>
      </div>
      <button @click="$emit('toggle-settings')" class="ml-2 flex items-center justify-center" style="width:32px; height:38px; border-radius:2px; background:rgba(255,255,255,.85); border:1px solid var(--BGCOLOR2);" title="Settings">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
        </svg>
      </button>
    </div>

    <div id="credits">Remote ID Monitor</div>

    <!-- 轨迹回放控制条 -->
    <div v-if="selectedDrone" id="replayBar">
      <span class="font-bold" style="color: var(--TXTCOLOR1);">{{ selectedDrone.uas_id || selectedDrone.mac }}</span>
      <input type="range" v-model.number="timelinePosition" min="0" max="100" class="w-32" @input="onTimelineChange" />
      <button @click="togglePlayback" class="sidebarButton" style="font-size:11px; padding:1px 6px;">{{ isPlaying ? '⏸' : '▶' }}</button>
      <button @click="stopPlayback" class="sidebarButton" style="font-size:11px; padding:1px 6px;">■</button>
      <button @click="$emit('update:selectedDrone', null)" class="text-xs" style="color: var(--TXTCOLOR2);">✕</button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import * as L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { shortenMac } from '@/utils/helpers'
import { fetchDroneTrajectory } from '@/utils/api'
import { useDroneStore } from '@/stores/drones'
import { setupLeafletIcons, createDroneIcon } from '@/utils/leaflet-setup'
import logger from '@/utils/logger'

setupLeafletIcons()

const props = defineProps({
  selectedDrone: { type: Object, default: null }
})
const emit = defineEmits(['update:selectedDrone', 'toggle-map', 'toggle-settings'])

const store = useDroneStore()
const mapContainer = ref(null)
const map = ref(null)
const droneMarkers = ref({})
const trajectoryLayers = ref({})
const playbackMarkers = ref({})

const currentMapType = ref('tianditu')
const timelinePosition = ref(0)
const isPlaying = ref(false)
let playbackInterval = null

const TDT_TOKEN = '66ad6f2f6216ab57401ed9907e94cd43'
const mapTypeOptions = [
  { key: 'tianditu', label: '天地图' },
  { key: 'tiandituImg', label: '天地图影像' },
  { key: 'amap', label: '高德' },
  { key: 'amapImg', label: '高德卫星' },
  { key: 'osm', label: 'OSM' }
]

const mapTypes = {
  tianditu: { 
    url: `https://t{s}.tianditu.gov.cn/vec_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=vec&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILECOL={x}&TILEROW={y}&TILEMATRIX={z}&tk=${TDT_TOKEN}`,
    annotationUrl: `https://t{s}.tianditu.gov.cn/cva_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=cva&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILECOL={x}&TILEROW={y}&TILEMATRIX={z}&tk=${TDT_TOKEN}`,
    subdomains: ['0','1','2','3','4','5','6','7'], attribution: '&copy; 天地图' 
  },
  tiandituImg: {
    url: `https://t{s}.tianditu.gov.cn/img_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=img&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILECOL={x}&TILEROW={y}&TILEMATRIX={z}&tk=${TDT_TOKEN}`,
    annotationUrl: `https://t{s}.tianditu.gov.cn/cva_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=cva&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILECOL={x}&TILEROW={y}&TILEMATRIX={z}&tk=${TDT_TOKEN}`,
    subdomains: ['0','1','2','3','4','5','6','7'], attribution: '&copy; 天地图'
  },
  // ✅ 高德矢量（直连官方 CDN，免 Key）
  amap: {
    url: 'https://webrd0{s}.is.autonavi.com/appmaptile?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}',
    subdomains: ['1', '2', '3', '4'], // 对应 webrd01 到 webrd04
    attribution: '&copy; 高德地图'
  },
  // ✅ 高德卫星影像（直连官方 CDN，免 Key）
  amapImg: {
    url: 'https://webst0{s}.is.autonavi.com/appmaptile?style=6&x={x}&y={y}&z={z}',
    subdomains: ['1', '2', '3', '4'], // 对应 webst01 到 webst04
    attribution: '&copy; 高德地图',
    annotationUrl: 'https://webst0{s}.is.autonavi.com/appmaptile?style=8&x={x}&y={y}&z={z}' // 卫星图上的文字标注层
  },
   // ✅ OSM (替换为国内能连上的镜像源，或者使用 Wikimedia 镜像)
  osm: {
    url: 'https://tile.openstreetmap.de/{z}/{x}/{y}.png', // 德国镜像，国内连通率较高
    // 备选镜像: 'https://maps.wikimedia.org/osm-intl/{z}/{x}/{y}.png'
    subdomains: ['a', 'b', 'c'], // openstreetmap.de 支持 a/b/c 子域名
    attribution: '&copy; OpenStreetMap contributors'
  }
}

// ---- 地图核心逻辑 ----
const initMap = () => {
  if (!mapContainer.value) return
  const rect = mapContainer.value.getBoundingClientRect()
  if (rect.width === 0 || rect.height === 0) {
    setTimeout(() => {
    if (map.value) {
      map.value.invalidateSize()
      // ✅ 核心修复：地图画布真正准备好后，再拉取第一波数据并画到地图上！
      refreshData() 
    }
  }, 300)
    return
  }
  if (map.value) map.value.remove()
  
  map.value = L.map(mapContainer.value, { zoomControl: false, attributionControl: true })
    .setView([23.14287, 113.26026], 18)
  L.control.zoom({ position: 'bottomright' }).addTo(map.value)
  updateMapLayer()
  setTimeout(() => map.value?.invalidateSize(), 300)
}

const updateMapLayer = () => {
  if (!map.value) return
  map.value.eachLayer(layer => {
    if (layer instanceof L.TileLayer) map.value.removeLayer(layer)
  })
  const config = mapTypes[currentMapType.value]
  L.tileLayer(config.url, { subdomains: config.subdomains, attribution: config.attribution }).addTo(map.value)
  if (config.annotationUrl) {
    L.tileLayer(config.annotationUrl, { subdomains: config.subdomains, attribution: '' }).addTo(map.value)
  }
}

// ✅ 补全：处理地图 Popup 里的点击事件
const handleMapClick = (e) => {
  const el = e.target
  if (el.classList.contains('view-details-btn')) {
    const popup = el.closest('.leaflet-popup-content')
    if (popup) {
      const nameEl = popup.querySelector('b')
      map.value?.closePopup()
      const name = nameEl?.textContent
      
      // 在 Store 中找到对应的无人机
      const drone = store.activeDrones.find(d => d.uas_id === name || d.mac === name)
      if (drone) {
        // 🎯 通知父组件 (App.vue) 选中这架无人机
        emit('update:selectedDrone', drone) 
      }
    }
  }
}

const changeMapType = (key) => {
  currentMapType.value = key
  updateMapLayer()
}

// ---- 标记更新逻辑 (修复核心) ----
const getDroneColor = (drone) => drone.standard === 'ASTM F3411-22a' ? 'blue' : 'green'

const createPopupContent = (drone) => `
  <div style="font-size:13px;">
    <b>${drone.uas_id || 'Unknown'}</b><br>
    MAC: ${drone.mac}<br>
    Alt: ${drone.altitude ? drone.altitude.toFixed(1) : 'N/A'}m<br>
    <button class="view-details-btn" data-mac="${drone.mac}" style="margin-top:4px; padding:2px 6px; background: var(--ACCENT); color:#fff; border:none; border-radius:2px; cursor:pointer; font-size:11px;">Details</button>
  </div>
`

// ✅ 批量更新 (HTTP 轮询触发)
const refreshData = async () => {
  try {
    // 1. 拉取数据并更新到 Store
    await store.loadActiveDrones()
    store.cleanStaleDrones()
    
    // ✅ 核心修复：直接使用 Store 里的响应式数据，绝不依赖返回值！
    const droneList = store.activeDrones 
    
    // 2. 更新地图标记
    updateDroneMarkers(droneList)
    
    await store.loadAlerts()
    
    // 3. 检查数据新鲜度
    if (droneList.length > 0) {
      const latestSeen = Math.max(...droneList.map(d => new Date(d.last_seen).getTime()))
      dataStaleWarning.value = (Date.now() - latestSeen) > 60000
    }
  } catch (e) { 
    logger.error('Refresh error:', e) 
  }
}

// ✅ 补全：批量更新地图标记 (由 watch 监听 Store 变化时调用)
const updateDroneMarkers = (drones) => {
  if (!map.value || !Array.isArray(drones)) return
  
  const activeMacs = new Set(drones.map(d => d.mac))
  
  // 1. 移除消失的无人机
  Object.keys(droneMarkers.value).forEach(mac => {
    if (!activeMacs.has(mac)) {
      map.value.removeLayer(droneMarkers.value[mac])
      delete droneMarkers.value[mac]
    }
  })

  // 2. 更新或创建 Marker
  drones.forEach(drone => {
    if (!drone.latitude || !drone.longitude) return
    const color = getDroneColor(drone)
    
    if (droneMarkers.value[drone.mac]) {
      const marker = droneMarkers.value[drone.mac]
      marker.setLatLng([drone.latitude, drone.longitude])
      marker.setIcon(createDroneIcon(color, 12))
      marker.setPopupContent(createPopupContent(drone))
    } else {
      const marker = L.marker([drone.latitude, drone.longitude], { 
        icon: createDroneIcon(color, 12) 
      }).addTo(map.value)
      marker.bindPopup(createPopupContent(drone))
      droneMarkers.value[drone.mac] = marker
    }
  })
}

// ✅ 单条更新 (WebSocket 触发，修复了之前“未定义”或“先于HTTP到达被忽略”的Bug)
const updateDroneMarker = (mac, droneData) => {
  if (!map.value) return
  
  // 严格校验坐标
  const lat = parseFloat(droneData.latitude)
  const lng = parseFloat(droneData.longitude)
  if (isNaN(lat) || isNaN(lng) || lat === 0 || lng === 0) return

  if (droneMarkers.value[mac]) {
    // 情况 A：Marker 已存在 -> 更新位置
    const marker = droneMarkers.value[mac]
    marker.setLatLng([lat, lng])
    marker.setPopupContent(createPopupContent(droneData, mac))
  } else {
    // ✅ 核心修复：情况 B：Marker 不存在 -> 直接创建！
    const color = getDroneColor(droneData)
    const marker = L.marker([lat, lng], {
      icon: createDroneIcon(color, 12)
    }).addTo(map.value)
    
    marker.bindPopup(createPopupContent(droneData, mac))
    droneMarkers.value[mac] = marker
  }
}

// ✅ 核心修复：监听 selectedDrone 的变化，自动移动地图
// ✅ 核心修复：使用 () => props.selectedDrone 来监听
watch(() => props.selectedDrone, (newDrone) => {
  if (newDrone && map.value) {
    const lat = parseFloat(newDrone.latitude)
    const lng = parseFloat(newDrone.longitude)
    
    // 严格校验坐标
    if (!isNaN(lat) && !isNaN(lng) && lat !== 0 && lng !== 0) {
      // 使用 flyTo 产生平滑的飞行动画
      map.value.flyTo([lat, lng], 16, {
        duration: 0.8 // 动画时间 0.8 秒
      })
      
      // 自动打开对应的 Marker 弹窗
      if (droneMarkers.value[newDrone.mac]) {
        droneMarkers.value[newDrone.mac].openPopup()
      }
    } else {
      logger.warn('⚠️ 选中无人机但坐标无效，无法移动地图:', { lat, lng, mac: newDrone.mac })
    }
  }
})

// 监听 Store 数据变化，自动批量更新
watch(() => store.activeDrones, (newDrones) => {
  updateDroneMarkers(newDrones)
}, { deep: true })


// ---- 轨迹与回放逻辑 ----
const clearTrajectories = () => {
  if (!map.value) return
  Object.values(trajectoryLayers.value).forEach(layer => map.value.removeLayer(layer))
  trajectoryLayers.value = {}
  Object.values(playbackMarkers.value).forEach(m => map.value.removeLayer(m))
  playbackMarkers.value = {}
}

// ---- 轨迹 ----
const showAllTrajectories = async () => {
  if (!map.value) return
  
  // 1. 先清除旧的轨迹
  clearTrajectories()
  
  // 2. 遍历当前活跃的无人机
  const drones = store.activeDrones
  if (!drones || drones.length === 0) {
    logger.warn('⚠️ 当前没有活跃的无人机，无法显示轨迹')
    return
  }

  logger.info(`🛤️ 开始加载 ${drones.length} 架无人机的轨迹...`)
  
  for (const drone of drones) {
    await showDroneTrajectory(drone.mac)
  }
}

const showDroneTrajectory = async (mac) => {
  if (!map.value) return
  
  try {
    const points = await fetchDroneTrajectory(mac)
    
    // 🎯 核心修复 1：严格的防御性校验
    // 防止后端返回 null、undefined 或非数组格式导致 points.length 报错
    if (!points || !Array.isArray(points) || points.length < 2) {
      logger.warn(`⚠️ 无人机 ${mac} 的轨迹数据不足 (点数: ${points ? points.length : 0})，跳过绘制`)
      return
    }

    // 🎯 核心修复 2：打印数据结构，方便你确认后端返回的字段名是否正确
    logger.info(`✅ 成功获取 ${mac} 的轨迹，样本点:`, points[0])

    // 提取经纬度 (请确保后端返回的字段名确实是 latitude 和 longitude)
    const latLngs = points.map(p => [p.latitude, p.longitude])
    
    // 🎯 核心修复 3：Leaflet 的 color 属性有时无法正确解析 CSS 变量 'var(--ACCENT)'
    // 将其替换为具体的十六进制颜色值（例如 Tailwind 的 blue-600: #3182ce）
    const polyline = L.polyline(latLngs, {
      color: '#3182ce', // 替换了 'var(--ACCENT)'
      weight: 3,        // 稍微加粗一点，更容易看清
      opacity: 0.7
    }).addTo(map.value)
    
    trajectoryLayers.value[mac] = polyline
    
  } catch (e) {
    logger.error(`❌ 获取无人机 ${mac} 轨迹失败:`, e)
  }
}

const loadDroneTrajectory = async (mac) => {
  try {
    return await fetchDroneTrajectory(mac) || []
  } catch (e) {
    logger.error('Load trajectory error:', e)
    return []
  }
}

let currentTrajectoryPoints = []
const togglePlayback = async () => {
  if (!props.selectedDrone) return
  if (isPlaying.value) { stopPlayback(); return }
  
  if (currentTrajectoryPoints.length === 0) {
    currentTrajectoryPoints = await loadDroneTrajectory(props.selectedDrone.mac)
  }
  if (currentTrajectoryPoints.length === 0) return
  
  isPlaying.value = true
  startPlayback()
}

const stopPlayback = () => {
  isPlaying.value = false
  if (playbackInterval) { clearInterval(playbackInterval); playbackInterval = null }
}

const onTimelineChange = () => {
  if (!isPlaying.value && currentTrajectoryPoints.length > 0 && props.selectedDrone) {
    const idx = Math.floor((timelinePosition.value / 100) * currentTrajectoryPoints.length)
    updatePlaybackMarker(idx)
  }
}

const startPlayback = () => {
  if (!map.value || !props.selectedDrone) return
  const points = currentTrajectoryPoints
  let idx = Math.floor((timelinePosition.value / 100) * points.length)
  
  playbackInterval = setInterval(() => {
    if (!isPlaying.value || !map.value) { stopPlayback(); return }
    if (idx < points.length) {
      updatePlaybackMarker(idx)
      timelinePosition.value = (idx / points.length) * 100
      idx++
    } else { stopPlayback() }
  }, 150)
}

const updatePlaybackMarker = (idx) => {
  const p = currentTrajectoryPoints[idx]
  if (!p) return
  if (playbackMarkers.value[props.selectedDrone.mac]) {
    map.value.removeLayer(playbackMarkers.value[props.selectedDrone.mac])
  }
  const marker = L.circleMarker([p.latitude, p.longitude], {
    radius: 6, color: 'var(--ACCENT)', fillColor: '#fff', fillOpacity: 1, weight: 2
  }).addTo(map.value)
  playbackMarkers.value[props.selectedDrone.mac] = marker
  map.value.setView([p.latitude, p.longitude], map.value.getZoom())
}

// 暴露单条更新方法给 App.vue 的 WebSocket 使用
// ✅ 暴露所有需要给父组件调用的方法
defineExpose({ 
  updateDroneMarker, 
  showAllTrajectories, 
  clearTrajectories 
})


// ---- 生命周期 ----
onMounted(() => {
  nextTick(() => {
    requestAnimationFrame(() => {
      initMap()
      
      // 仅保留地图相关的点击事件监听（用于处理 Popup 里的 Details 按钮）
      if (mapContainer.value) {
        mapContainer.value.addEventListener('click', handleMapClick)
      }
    })
  })
  
  // ❌ 彻底删除以下两行！它们不属于地图组件的职责
  // initWebSocket()
  // pollInterval = setInterval(refreshData, 5000)
})

onUnmounted(() => {
  stopPlayback()
  
  // 清理地图点击事件
  if (mapContainer.value) {
    mapContainer.value.removeEventListener('click', handleMapClick)
  }
  
  // 清理地图实例
  if (map.value) {
    map.value.eachLayer(l => map.value.removeLayer(l))
    map.value.remove()
    map.value = null
  }
  
  // ❌ 彻底删除以下清理代码！
  // if (playbackInterval) clearInterval(playbackInterval)
  // if (pollInterval) clearInterval(pollInterval)
  // if (ws) { ws.close(); ws = null }
})
</script>

<style scoped>
/* (保持与你知识库中相同的 CSS，此处省略以节省篇幅，请直接复用你原有的 #map_container 等样式) */
#map_container { flex: 1 1 auto; position: relative; height: 100%; min-height: 0; }
#map_canvas { position: absolute; inset: 0; }
#map_canvas :deep(.leaflet-container) { height: 100% !important; width: 100% !important; }
#header_side { position: absolute; top: 6px; right: 6px; z-index: 999; display: flex; align-items: center; }
.toggle_sidebar_button { display: flex; align-items: center; justify-content: center; width: 32px; height: 38px; border-radius: 2px; background: rgba(255,255,255,.85); border: 1px solid var(--BGCOLOR2); cursor: pointer; color: var(--TXTCOLOR2); }
#credits { position: absolute; bottom: 4px; left: 50%; transform: translateX(-50%); opacity: 0.7; z-index: 99; font-size: var(--FS1); color: var(--TXTCOLOR1); pointer-events: none; }
#replayBar { position: absolute; bottom: 28px; left: 50%; transform: translateX(-50%); z-index: 100; display: flex; align-items: center; gap: 6px; padding: 4px 8px; border-radius: 2px; box-shadow: 0 2px 6px rgba(0,0,0,.15); background: var(--BGCOLOR1); border: 1px solid var(--BGCOLOR2); font-size: var(--FS1); }
</style>