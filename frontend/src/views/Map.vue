<template>
  <div class="map-container">
    <div ref="mapContainer" class="h-full w-full"></div>
  </div>
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

  map = L.map(mapContainer.value).setView([23.14287, 113.26026], 12)
  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
  }).addTo(map)

  const marker = L.marker([23.14287, 113.26026]).addTo(map)
  marker.bindPopup('Remote ID Test Location').openPopup()
})

onUnmounted(() => {
  if (map) map.remove()
})
</script>

<style scoped>
.map-container {
  height: 600px;
  width: 100%;
}
</style>
