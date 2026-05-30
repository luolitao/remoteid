<!-- tar1090-style standalone map component -->
<template>
  <div ref="mapContainer" class="h-full w-full"></div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import * as L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { setupLeafletIcons } from '@/utils/leaflet-setup'

setupLeafletIcons()

const mapContainer = ref(null)
let map = null

onMounted(() => {
  if (!mapContainer.value) return
  // 确保容器有尺寸再初始化
  const rect = mapContainer.value.getBoundingClientRect()
  if (rect.width === 0 || rect.height === 0) {
    setTimeout(() => {
      if (mapContainer.value) {
        map = L.map(mapContainer.value, { zoomControl: false }).setView([23.14287, 113.26026], 10)
        L.control.zoom({ position: 'bottomright' }).addTo(map)
        L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
          attribution: '&copy; OpenStreetMap'
        }).addTo(map)
      }
    }, 100)
    return
  }
  map = L.map(mapContainer.value, { zoomControl: false }).setView([23.14287, 113.26026], 10)
  L.control.zoom({ position: 'bottomright' }).addTo(map)
  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    attribution: '&copy; OpenStreetMap'
  }).addTo(map)
})

onUnmounted(() => {
  if (map) { map.remove(); map = null }
})
</script>

<style scoped>
::deep(.leaflet-container) {
  height: 100%;
  width: 100%;
}
</style>
