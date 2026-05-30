<template>
  <div class="map-container">
    <div ref="mapContainer" class="h-full w-full"></div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import * as L from 'leaflet'
import 'leaflet/dist/leaflet.css'

// 修复 Leaflet 图标路径问题
delete L.Icon.Default.prototype._getIconUrl
L.Icon.Default.mergeOptions({
  iconRetinaUrl: require('leaflet/dist/images/marker-icon-2x.png'),
  iconUrl: require('leaflet/dist/images/marker-icon.png'),
  shadowUrl: require('leaflet/dist/images/marker-shadow.png')
})

const mapContainer = ref(null)
let map = null

onMounted(() => {
  if (!mapContainer.value) return
  
  // 初始化地图
  map = L.map(mapContainer.value).setView([23.14287, 113.26026], 12)
  
  // 添加底图
  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
  }).addTo(map)
  
  // 添加示例标记
  const marker = L.marker([23.14287, 113.26026]).addTo(map)
  marker.bindPopup('Remote ID Test Location').openPopup()
})

onUnmounted(() => {
  if (map) {
    map.remove()
  }
})
</script>

<style scoped>
.map-container {
  height: 600px;
  width: 100%;
}
</style>
