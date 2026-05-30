<!-- src/App.vue -->
<template>
  <div class="min-h-screen bg-gray-100">
    <header class="bg-blue-600 text-white p-4 shadow-md">
      <div class="container mx-auto flex justify-between items-center">
        <h1 class="text-2xl font-bold">🛰️ Remote ID Monitor</h1>
        <div class="flex items-center space-x-4">
          <span class="bg-green-500 px-2 py-1 rounded-full animate-pulse"></span>
          <span>Connected</span>
          <button @click="toggleFullscreen" class="p-2 hover:bg-blue-700 rounded">
            <FullScreenIcon v-if="!isFullscreen" />
            <MinimizeIcon v-else />
          </button>
        </div>
      </div>
    </header>

    <div class="container mx-auto p-4">
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <!-- 地图区域 -->
        <div class="lg:col-span-2 bg-white rounded-lg shadow-md overflow-hidden">
          <div ref="mapContainer" class="h-[600px] w-full"></div>
          
          <!-- 轨迹控制 -->
          <div class="p-4 border-t">
            <div class="flex items-center space-x-4">
              <div class="flex-1">
                <label class="block text-sm font-medium text-gray-700 mb-1">轨迹回放</label>
                <input type="range" v-model="timelinePosition" min="0" max="100" class="w-full">
              </div>
              <button @click="togglePlayback" class="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700">
                {{ isPlaying ? '⏸ Pause' : '▶ Play' }}
              </button>
              <button @click="exportData" class="px-4 py-2 bg-green-600 text-white rounded hover:bg-green-700">
                📤 Export CSV
              </button>
            </div>
          </div>
        </div>

        <!-- 侧边栏 -->
        <div class="space-y-6">
          <!-- 无人机列表 -->
          <div class="bg-white rounded-lg shadow-md p-4">
            <h2 class="text-lg font-semibold mb-4 flex items-center">
              <DroneIcon class="mr-2" /> Active Drones ({{ activeDrones.length }})
            </h2>
            <div class="space-y-3 max-h-[400px] overflow-y-auto">
              <div 
                v-for="drone in activeDrones" 
                :key="drone.mac"
                class="border rounded-lg p-3 hover:bg-blue-50 cursor-pointer"
                @click="selectDrone(drone)"
              >
                <div class="font-bold text-blue-600">{{ drone.uas_id }}</div>
                <div class="text-sm text-gray-600">MAC: {{ drone.mac }}</div>
                <div class="flex justify-between mt-1 text-xs">
                  <span>{{ drone.ua_type }}</span>
                  <span :class="{
                    'text-green-600': isRecent(drone.last_seen),
                    'text-gray-400': !isRecent(drone.last_seen)
                  }">
                    {{ timeAgo(drone.last_seen) }}
                  </span>
                </div>
              </div>
              <div v-if="activeDrones.length === 0" class="text-center text-gray-500 py-4">
                No drones detected in the last 5 minutes
              </div>
            </div>
          </div>

          <!-- 警报面板 -->
          <div class="bg-white rounded-lg shadow-md p-4">
            <h2 class="text-lg font-semibold mb-4 flex items-center">
              <AlertIcon class="mr-2 text-red-500" /> Alerts
            </h2>
            <div class="space-y-3 max-h-[300px] overflow-y-auto">
              <div 
                v-for="(alert, index) in alerts" 
                :key="index" 
                class="border-l-4 border-red-500 bg-red-50 p-2 mb-2"
              >
                <div class="font-medium">{{ alert.type }}</div>
                <div class="text-sm text-gray-700">{{ alert.message }}</div>
                <div class="text-xs text-gray-500 mt-1">{{ alert.timestamp }}</div>
              </div>
              <div v-if="alerts.length === 0" class="text-center text-gray-500 py-2">
                No active alerts
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import * as L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import axios from 'axios'

// 状态
const mapContainer = ref(null)
const map = ref(null)
const droneMarkers = ref({})
const activeDrones = ref([])
const alerts = ref([])
const timelinePosition = ref(100)
const isPlaying = ref(false)
const isFullscreen = ref(false)
const selectedDrone = ref(null)

// WebSocket
let ws = null

// 初始化地图
const initMap = () => {
  if (!mapContainer.value) return
  
  // 移除旧地图
  if (map.value) {
    map.value.remove()
  }
  
  // 创建新地图
  map.value = L.map(mapContainer.value).setView([23.14287, 113.26026], 12)
  
  // 添加底图
  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
  }).addTo(map.value)
}

// 连接 WebSocket
const connectWebSocket = () => {
  ws = new WebSocket('ws://raspberrypi.local:8000/ws')
  
  ws.onmessage = (event) => {
    const data = JSON.parse(event.data)
    if (data.type === 'drone_update') {
      updateDroneMarker(data.mac, data.data)
    }
  }
  
  ws.onclose = () => {
    console.log('WebSocket disconnected, reconnecting...')
    setTimeout(connectWebSocket, 3000)
  }
}

// 更新无人机标记
const updateDroneMarker = (mac, droneData) => {
  if (!map.value) return
  
  // 更新或创建标记
  if (droneMarkers.value[mac]) {
    const marker = droneMarkers.value[mac]
    marker.setLatLng([droneData.latitude, droneData.longitude])
    marker.setPopupContent(createPopupContent(droneData, mac))
  } else {
    const marker = L.marker([droneData.latitude, droneData.longitude], {
      icon: L.divIcon({
        html: `<div class="bg-red-500 rounded-full w-3 h-3 border-2 border-white"></div>`,
        className: '',
        iconSize: [10, 10]
      })
    }).addTo(map.value)
    marker.bindPopup(createPopupContent(droneData, mac))
    droneMarkers.value[mac] = marker
  }
}

// 创建弹出内容
const createPopupContent = (droneData, mac) => {
  return `
    <div class="p-2">
      <h3 class="font-bold">${droneData.uas_id}</h3>
      <div>MAC: ${mac}</div>
      <div>Type: ${droneData.ua_type}</div>
      <div>Alt: ${droneData.altitude?.toFixed(1) || 'N/A'}m</div>
      <div class="mt-2">
        <button onclick="showTrajectory('${mac}')" class="px-2 py-1 bg-blue-500 text-white rounded text-sm">
          Show Trajectory
        </button>
      </div>
    </div>
  `
}

// 获取活跃无人机
const fetchActiveDrones = async () => {
  try {
    const response = await axios.get('http://raspberrypi.local:8000/api/drones')
    activeDrones.value = response.data
  } catch (error) {
    console.error('Error fetching drones:', error)
  }
}

// 获取警报
const fetchAlerts = async () => {
  try {
    const response = await axios.get('http://raspberrypi.local:8000/api/alerts?limit=10')
    alerts.value = response.data
  } catch (error) {
    console.error('Error fetching alerts:', error)
  }
}

// 实用函数
const timeAgo = (timestamp) => {
  const now = new Date()
  const then = new Date(timestamp)
  const diff = Math.floor((now - then) / 1000)
  
  if (diff < 60) return `${diff}s ago`
  if (diff < 3600) return `${Math.floor(diff/60)}m ago`
  return `${Math.floor(diff/3600)}h ago`
}

const isRecent = (timestamp) => {
  const now = new Date()
  const then = new Date(timestamp)
  return (now - then) < 300000 // 5 minutes
}

// 生命周期
onMounted(() => {
  initMap()
  connectWebSocket()
  
  // 初始获取数据
  fetchActiveDrones()
  fetchAlerts()
  
  // 设置定时刷新
  const interval = setInterval(() => {
    fetchActiveDrones()
    fetchAlerts()
  }, 5000)
  
  onUnmounted(() => clearInterval(interval))
})

// 全屏切换
const toggleFullscreen = () => {
  if (!document.fullscreenElement) {
    document.documentElement.requestFullscreen()
    isFullscreen.value = true
  } else {
    if (document.exitFullscreen) {
      document.exitFullscreen()
      isFullscreen.value = false
    }
  }
}

// 选择无人机
const selectDrone = (drone) => {
  selectedDrone.value = drone
  if (map.value && drone.latitude && drone.longitude) {
    map.value.setView([drone.latitude, drone.longitude], 14)
  }
}

// 切换播放
const togglePlayback = () => {
  isPlaying.value = !isPlaying.value
  if (isPlaying.value) {
    startPlayback()
  }
}

// 开始播放
const startPlayback = () => {
  if (!isPlaying.value) return
  
  timelinePosition.value += 1
  if (timelinePosition.value > 100) {
    timelinePosition.value = 0
  }
  
  setTimeout(startPlayback, 100)
}

// 导出数据
const exportData = async () => {
  try {
    const response = await axios.get('http://raspberrypi.local:8000/api/export/csv', {
      responseType: 'blob'
    })
    
    const url = window.URL.createObjectURL(new Blob([response.data]))
    const link = document.createElement('a')
    link.href = url
    link.setAttribute('download', `remoteid_export_${new Date().toISOString().split('T')[0]}.csv`)
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  } catch (error) {
    console.error('Export failed:', error)
  }
}
</script>

<style scoped>
/* 全屏样式 */
.fullscreen {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  z-index: 1000;
  background: white;
}

/* 地图容器在全屏时调整 */
.fullscreen .map-container {
  height: calc(100vh - 64px) !important;
}
</style>
