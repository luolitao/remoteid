<template>
  <div ref="mapContainer" class="h-full w-full overflow-hidden"></div>
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
  showTrajectory: Boolean
})

const mapContainer = ref(null)
let map = null
let droneMarker = null
let ceRing = null

const initMap = () => {
  if (!mapContainer.value) return
  if (map) map.remove()

  map = L.map(mapContainer.value, { zoomControl: true }).setView([23.14287, 113.26026], 14)

  // Dark CartoDB tiles for consistent dark theme
  L.tileLayer(
    'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png',
    {
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OSM</a> &copy; <a href="https://carto.com/">CARTO</a>',
      subdomains: ['a', 'b', 'c', 'd'],
      maxZoom: 19,
    }
  ).addTo(map)
}

const updateMarker = () => {
  if (!map || !props.drone) return

  const { latitude, longitude, altitude, uas_id, standard, last_seen, caa_compliant } = props.drone
  if (!latitude || !longitude || isNaN(latitude) || isNaN(longitude)) return

  const color = caa_compliant ? 'green' : 'red'

  if (droneMarker) {
    droneMarker.setLatLng([latitude, longitude])
    droneMarker.setIcon(createDroneIcon(color, 14, true))
    droneMarker.setPopupContent(createPopupContent())
  } else {
    droneMarker = L.marker([latitude, longitude], {
      icon: createDroneIcon(color, 14, true)
    }).addTo(map)
    droneMarker.bindPopup(createPopupContent())
  }

  // CE uncertainty ring (inspired by Phantom-Proof)
  if (ceRing) {
    ceRing.setLatLng([latitude, longitude])
  } else {
    ceRing = L.circle([latitude, longitude], {
      radius: 50,
      color: caa_compliant ? '#19d27a' : '#ff4d6d',
      weight: 1,
      opacity: 0.45,
      fillColor: caa_compliant ? '#19d27a' : '#ff4d6d',
      fillOpacity: 0.04,
      dashArray: '3 4',
      interactive: false,
    }).addTo(map)
  }

  map.setView([latitude, longitude], 16)
}

const createPopupContent = () => {
  if (!props.drone) return ''
  const { uas_id, altitude, standard, last_seen, caa_compliant } = props.drone

  const statusColor = caa_compliant ? '#19d27a' : '#ff4d6d'
  const statusText = caa_compliant ? 'CAA COMPLIANT' : 'NON-COMPLIANT'

  return `
    <div style="font-family:ui-monospace,monospace;font-size:11px;color:#b3becf;min-width:180px;">
      <b style="color:#e8eef7;font-size:12px;">${uas_id || 'Unknown'}</b><br>
      <hr style="border:none;border-top:1px solid #1a2027;margin:6px 0">
      <div style="display:grid;grid-template-columns:auto 1fr;gap:2px 8px;">
        <span style="color:#687586;">Alt:</span><span style="color:#e8eef7;">${altitude ? altitude.toFixed(1) : '—'} m</span>
        <span style="color:#687586;">Std:</span><span style="color:#e8eef7;">${standard || '—'}</span>
        <span style="color:#687586;">Seen:</span><span style="color:#e8eef7;">${timeAgo(last_seen)}</span>
        <span style="color:#687586;">Status:</span><span style="color:${statusColor};font-weight:700;">${statusText}</span>
      </div>
    </div>
  `
}

onMounted(() => {
  initMap()
  updateMarker()
})

onUnmounted(() => {
  if (map) {
    map.remove()
    map = null
  }
  droneMarker = null
  ceRing = null
})

watch(() => props.drone, () => {
  updateMarker()
}, { deep: true })

watch(() => props.showTrajectory, (newVal) => {
  if (newVal && props.drone && map) {
    map.setView([props.drone.latitude, props.drone.longitude], 14)
  }
})
</script>

<style scoped>
:deep(.leaflet-container) {
  height: 100%;
  width: 100%;
  background: #0e1117 !important;
}
</style>
