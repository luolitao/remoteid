<!-- tar1090-style drone map component -->
<template>
  <div ref="mapContainer" class="h-full w-full"></div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import * as L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { setupLeafletIcons, createDroneIcon } from '@/utils/leaflet-setup'
import { timeAgo } from '@/utils/helpers'

setupLeafletIcons()

const props = defineProps({
  drone: Object,
  showTrajectory: Boolean,
})

const mapContainer = ref(null)
let map = null
let droneMarker = null

const initMap = () => {
  if (!mapContainer.value) return
  if (map) map.remove()

  map = L.map(mapContainer.value, { zoomControl: false }).setView([23.14287, 113.26026], 14)
  L.control.zoom({ position: 'bottomright' }).addTo(map)
  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    attribution: '&copy; OpenStreetMap'
  }).addTo(map)
}

const updateMarker = () => {
  if (!map || !props.drone) return
  const { latitude, longitude, altitude, uas_id, standard, last_seen } = props.drone
  if (!latitude || !longitude) return

  const color = standard === 'ASTM F3411-22a' ? 'blue' : 'green'

  if (droneMarker) {
    droneMarker.setLatLng([latitude, longitude])
    droneMarker.setIcon(createDroneIcon(color, 14))
    droneMarker.setPopupContent(createPopup())
  } else {
    droneMarker = L.marker([latitude, longitude], {
      icon: createDroneIcon(color, 14)
    }).addTo(map)
    droneMarker.bindPopup(createPopup())
  }

  map.setView([latitude, longitude], 16)
}

const createPopup = () => {
  if (!props.drone) return ''
  const { uas_id, altitude, standard, last_seen } = props.drone
  return `
    <div style="font-size:13px;">
      <b style="color: var(--TXTCOLOR1);">${uas_id || 'Unknown'}</b><br>
      Altitude: ${altitude ? altitude.toFixed(1) : 'N/A'}m<br>
      Standard: ${standard || 'N/A'}<br>
      Last Seen: ${timeAgo(last_seen)}
    </div>
  `
}

onMounted(() => { initMap(); updateMarker() })
onUnmounted(() => { if (map) { map.remove(); map = null } })

watch(() => props.drone, () => updateMarker(), { deep: true })
watch(() => props.showTrajectory, (v) => {
  if (v && props.drone && map) {
    map.setView([props.drone.latitude, props.drone.longitude], 14)
  }
})
</script>

<style scoped>
::deep(.leaflet-container) {
  height: 100%;
  width: 100%;
}
</style>
