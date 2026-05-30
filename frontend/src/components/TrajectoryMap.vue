<template>
  <div ref="mapContainer" class="h-full w-full"></div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import * as L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { setupLeafletIcons } from '@/utils/leaflet-setup'

setupLeafletIcons()

const props = defineProps({
  positions: {
    type: Array,
    default: () => []
  }
})

const mapContainer = ref(null)
let map = null

const initMap = () => {
  if (!mapContainer.value) return
  map = L.map(mapContainer.value).setView([23.14287, 113.26026], 12)

  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
  }).addTo(map)

  if (props.positions?.length > 0) drawTrajectory()
}

const drawTrajectory = () => {
  if (!map || !props.positions?.length) return

  // 清除现有图层（保留 tile layer）
  map.eachLayer(layer => {
    if (layer instanceof L.Polyline || (layer instanceof L.CircleMarker)) {
      map.removeLayer(layer)
    }
  })

  const latLngs = props.positions.map(pos => [pos.latitude, pos.longitude])
  L.polyline(latLngs, {
    color: '#3388ff', weight: 3, opacity: 0.7
  }).addTo(map)

  if (latLngs.length > 0) {
    L.circleMarker(latLngs[0], {
      radius: 5, color: 'green', fillColor: '#3388ff', fillOpacity: 1
    }).addTo(map).bindPopup(`Start: ${props.positions[0].timestamp}`)

    L.circleMarker(latLngs[latLngs.length - 1], {
      radius: 5, color: 'red', fillColor: '#3388ff', fillOpacity: 1
    }).addTo(map).bindPopup(`End: ${props.positions[latLngs.length - 1].timestamp}`)
  }

  const bounds = L.latLngBounds(latLngs)
  map.fitBounds(bounds, { padding: [50, 50] })
}

onMounted(() => initMap())

onUnmounted(() => {
  if (map) { map.remove(); map = null }
})

watch(() => props.positions, () => {
  if (map) drawTrajectory()
})
</script>

<style scoped>
div {
  height: 100%;
  width: 100%;
}
</style>
