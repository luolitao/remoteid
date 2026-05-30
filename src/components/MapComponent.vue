<!-- src/components/MapComponent.vue — Generic dark-themed map component -->
<template>
  <div ref="mapContainer" class="w-full h-full min-h-[300px]"></div>
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
  if (!mapContainer.value) {
    console.error('Map container not found')
    return
  }

  map = L.map(mapContainer.value, { zoomControl: true }).setView([23.14287, 113.26026], 12)

  // Dark CartoDB tiles
  L.tileLayer(
    'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png',
    {
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OSM</a> &copy; <a href="https://carto.com/">CARTO</a>',
      subdomains: ['a', 'b', 'c', 'd'],
      maxZoom: 19,
    }
  ).addTo(map)

  // Test marker with glow effect
  const icon = L.divIcon({
    className: '',
    html: `<div style="width:14px;height:14px;border-radius:50%;background:#56a8ff;border:2px solid #fff;box-shadow:0 0 12px #56a8ff;"></div>`,
    iconSize: [14, 14],
    iconAnchor: [7, 7],
  })
  L.marker([23.14287, 113.26026], { icon }).addTo(map)
    .bindPopup('<span style="font-family:ui-monospace,monospace;font-size:11px;">Remote ID Test Location</span>')
    .openPopup()
})

onUnmounted(() => {
  if (map) map.remove()
})
</script>

<style scoped>
:deep(.leaflet-container) {
  height: 100%;
  width: 100%;
  background: #0e1117 !important;
}
</style>
