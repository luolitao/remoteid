<!-- tar1090-style trajectory map -->
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
  positions: { type: Array, default: () => [] }
})

const mapContainer = ref(null)
let map = null

const initMap = () => {
  if (!mapContainer.value) return
  map = L.map(mapContainer.value, { zoomControl: false }).setView([23.14287, 113.26026], 12)
  L.control.zoom({ position: 'bottomright' }).addTo(map)
  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    attribution: '&copy; OpenStreetMap'
  }).addTo(map)
  if (props.positions?.length > 0) drawTrajectory()
}

const drawTrajectory = () => {
  if (!map || !props.positions?.length) return

  map.eachLayer(layer => {
    if (layer instanceof L.Polyline || layer instanceof L.CircleMarker) {
      map.removeLayer(layer)
    }
  })

  const latLngs = props.positions.map(p => [p.latitude, p.longitude])
  L.polyline(latLngs, {
    color: 'var(--ACCENT)', weight: 3, opacity: 0.7
  }).addTo(map)

  if (latLngs.length > 0) {
    L.circleMarker(latLngs[0], {
      radius: 4, color: '#19d27a', fillColor: '#19d27a', fillOpacity: 1
    }).addTo(map).bindPopup(`Start: ${props.positions[0].timestamp}`)

    L.circleMarker(latLngs[latLngs.length - 1], {
      radius: 4, color: '#ff4d6d', fillColor: '#ff4d6d', fillOpacity: 1
    }).addTo(map).bindPopup(`End: ${props.positions[latLngs.length - 1].timestamp}`)
  }

  map.fitBounds(L.latLngBounds(latLngs), { padding: [30, 30] })
}

onMounted(() => initMap())
onUnmounted(() => { if (map) { map.remove(); map = null } })
watch(() => props.positions, () => { if (map) drawTrajectory() })
</script>

<style scoped>
::deep(.leaflet-container) {
  height: 100%;
  width: 100%;
}
</style>
