<!-- src/App.vue — Dark-themed tactical console, inspired by Phantom-Proof -->
<template>
  <div class="h-screen flex flex-col bg-void overflow-hidden">
    <!-- ================================================================== -->
    <!-- TOP BAR — compact: title + tagline + stats + actions               -->
    <!-- ================================================================== -->
    <header class="bg-surface-1 border-b border-hairline-strong px-4 py-2 flex-shrink-0">
      <div class="flex items-center justify-between gap-4 mb-2">
        <!-- Brand -->
        <div class="flex items-baseline gap-3 min-w-0">
          <h1 class="text-lg font-extrabold text-fg-hi tracking-tight whitespace-nowrap">
            Remote ID <span class="text-accent">Monitor</span>
          </h1>
          <span class="text-xs text-fg-mute hidden sm:inline truncate">
            Real-time drone tracking &amp; Remote ID verification
          </span>
        </div>

        <!-- Actions -->
        <div class="flex items-center gap-2 flex-shrink-0">
          <button
            @click="refreshData"
            class="px-3 py-1.5 text-xs font-bold tracking-wider
                   bg-accent text-void border border-accent
                   hover:-translate-y-px active:translate-y-0
                   transition-transform duration-120 cursor-pointer"
            title="Refresh data"
          >
            REFRESH
          </button>
          <button
            @click="exportData"
            class="px-3 py-1.5 text-xs font-bold tracking-wider
                   bg-transparent text-fg-mid border border-hairline-strong
                   hover:text-fg-hi hover:border-accent
                   transition-all duration-150 cursor-pointer"
            title="Export all drone data"
          >
            EXPORT
          </button>
          <button
            @click="toggleFullscreen"
            class="px-3 py-1.5 text-xs font-bold tracking-wider
                   bg-transparent text-fg-mid border border-hairline-strong
                   hover:text-fg-hi hover:border-accent
                   transition-all duration-150 cursor-pointer"
            :title="isFullscreen ? 'Exit fullscreen' : 'Fullscreen'"
          >
            <FullScreenIcon v-if="!isFullscreen" class="w-4 h-4" />
            <MinimizeIcon v-else class="w-4 h-4" />
          </button>
        </div>
      </div>

      <!-- Summary Cards Row — 5 equal columns -->
      <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2">
        <!-- Active Drones -->
        <div class="card-summary border-l-accent">
          <span class="card-label text-accent">ACTIVE</span>
          <span class="card-num">{{ store.activeDrones.length }}</span>
        </div>
        <!-- Confirmed (compliant) -->
        <div class="card-summary border-l-confirmed">
          <span class="card-label text-confirmed">COMPLIANT</span>
          <span class="card-num">{{ compliantCount }}</span>
        </div>
        <!-- Non-compliant -->
        <div class="card-summary border-l-deception">
          <span class="card-label text-deception">NON‑COMPLIANT</span>
          <span class="card-num">{{ nonCompliantCount }}</span>
        </div>
        <!-- Alerts -->
        <div class="card-summary border-l-phantom">
          <span class="card-label text-phantom">ALERTS</span>
          <span class="card-num">{{ store.alertCount }}</span>
        </div>
        <!-- Status -->
        <div class="card-summary border-l-accent">
          <span class="card-label text-fg-mute">STATUS</span>
          <div class="flex items-center gap-1.5">
            <span class="status-dot" :class="wsConnected ? 'live' : 'error'"></span>
            <span class="text-xs font-mono font-bold text-fg-hi">
              {{ wsConnected ? 'LIVE' : 'OFFLINE' }}
            </span>
          </div>
        </div>
      </div>
    </header>

    <!-- ================================================================== -->
    <!-- MAIN WORK AREA: left panel (1) + map (1.4)                         -->
    <!-- ================================================================== -->
    <div class="flex-1 grid grid-cols-1 lg:grid-cols-[0.95fr_1.4fr] gap-px bg-hairline-strong min-h-0">
      <!-- Left Column: drone list (top) + alerts (bottom), stacked -->
      <div class="grid grid-rows-[1.35fr_1fr] gap-px bg-hairline-strong min-h-0 min-w-0">
        <!-- Drone List Panel -->
        <section class="bg-surface-1 flex flex-col min-h-0 overflow-hidden">
          <div class="pane-head">
            <div class="flex items-center gap-2">
              <DroneIcon class="w-4 h-4 text-accent" />
              <span>ACTIVE DRONES</span>
              <b class="text-fg-hi">{{ store.activeDrones.length }}</b>
            </div>
            <span class="text-fg-dim">recent 30 min</span>
          </div>
          <div class="flex-1 overflow-y-auto p-2 space-y-1">
            <div
              v-for="drone in store.activeDrones"
              :key="drone.mac"
              class="drone-item group"
              :data-status="drone.caa_compliant ? 'compliant' : 'noncompliant'"
              @click="selectDrone(drone)"
            >
              <div class="flex items-center justify-between">
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2">
                    <span class="text-sm font-bold text-fg-hi truncate">
                      {{ drone.uas_id || 'Unknown ID' }}
                    </span>
                    <span
                      class="text-[10px] px-1.5 py-0.5 rounded font-mono font-bold tracking-wider flex-shrink-0"
                      :class="drone.caa_compliant
                        ? 'bg-confirmed/15 text-confirmed'
                        : 'bg-deception/15 text-deception'"
                    >
                      {{ drone.caa_compliant ? 'CAA ✓' : 'CAA ✗' }}
                    </span>
                  </div>
                  <div class="text-[11px] text-fg-mute font-mono mt-0.5">
                    {{ shortenMac(drone.mac, true) }}
                    <span class="mx-1.5 text-fg-dim">·</span>
                    {{ drone.ua_type || 'Unknown' }}
                  </div>
                </div>
                <span
                  class="text-[10px] font-mono flex-shrink-0 ml-2"
                  :class="isRecent(drone.last_seen) ? 'text-confirmed' : 'text-fg-dim'"
                >
                  {{ timeAgo(drone.last_seen) }}
                </span>
              </div>
            </div>
            <div
              v-if="store.activeDrones.length === 0"
              class="text-center text-fg-dim font-mono text-xs py-12"
            >
              No active drones detected
            </div>
          </div>
        </section>

        <!-- Alerts Panel -->
        <section class="bg-surface-1 flex flex-col min-h-0 overflow-hidden">
          <div class="pane-head">
            <div class="flex items-center gap-2">
              <AlertIcon class="w-4 h-4" :class="store.alertCount > 0 ? 'text-deception' : 'text-fg-mute'" />
              <span>ALERTS</span>
              <b class="text-fg-hi">{{ store.alertCount }}</b>
            </div>
            <span class="text-fg-dim">{{ alertSummary }}</span>
          </div>
          <div class="flex-1 overflow-y-auto p-2 space-y-1.5">
            <div
              v-for="(alert, index) in store.alerts.filter(a => a)"
              :key="index"
              class="alert-item"
              :data-severity="getAlertSeverity(alert)"
            >
              <div class="flex items-center justify-between mb-1">
                <span class="text-xs font-bold text-fg-hi">
                  {{ alert.type || 'Unknown Alert' }}
                </span>
                <span
                  class="text-[10px] px-1.5 py-0.5 rounded font-mono font-bold tracking-wider"
                  :class="getAlertBadgeClass(alert)"
                >
                  {{ getAlertSeverity(alert).toUpperCase() }}
                </span>
              </div>
              <div class="text-[11px] text-fg-mid leading-relaxed">
                {{ alert.message || 'No details' }}
              </div>
              <div class="text-[10px] text-fg-dim font-mono mt-1">
                {{ alert.timestamp || '—' }}
              </div>
            </div>
            <div
              v-if="!store.alerts.length"
              class="text-center text-fg-dim font-mono text-xs py-10"
            >
              No active alerts
            </div>
          </div>
        </section>
      </div>

      <!-- Right Column: Map -->
      <section class="bg-surface-1 flex flex-col min-h-0 min-w-0 overflow-hidden relative">
        <!-- Map Controls Bar -->
        <div class="pane-head">
          <div class="flex items-center gap-2">
            <span>MAP</span>
            <div class="flex gap-1">
              <button
                v-for="mt in mapTypeOptions"
                :key="mt.key"
                @click="changeMapType(mt.key)"
                class="text-[10px] px-2 py-1 font-mono font-bold tracking-wider transition-colors"
                :class="currentMapType === mt.key
                  ? 'bg-accent text-void'
                  : 'bg-surface-2 text-fg-mute hover:text-fg-hi border border-hairline'"
              >
                {{ mt.label }}
              </button>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <!-- Playback Controls -->
            <button
              @click="togglePlayback"
              class="text-[10px] px-2 py-1 font-mono font-bold bg-accent text-void hover:-translate-y-px transition-transform cursor-pointer"
              :disabled="!selectedDrone"
              :class="{ 'opacity-50 cursor-not-allowed': !selectedDrone }"
            >
              {{ isPlaying ? '⏸ PAUSE' : '▶ PLAY' }}
            </button>
            <button
              @click="stopPlayback"
              class="text-[10px] px-2 py-1 font-mono font-bold bg-surface-2 text-fg-mute border border-hairline hover:text-fg-hi cursor-pointer"
            >
              ■ STOP
            </button>
            <button
              @click="showAllTrajectories"
              class="text-[10px] px-2 py-1 font-mono font-bold bg-surface-2 text-accent border border-hairline hover:border-accent cursor-pointer"
            >
              TRAILS
            </button>
            <button
              @click="clearTrajectories"
              class="text-[10px] px-2 py-1 font-mono font-bold bg-surface-2 text-fg-mute border border-hairline hover:text-fg-hi cursor-pointer"
            >
              CLEAR
            </button>
          </div>
        </div>

        <!-- Map Container -->
        <div ref="mapContainer" id="main-map" class="flex-1 w-full min-h-0"></div>

        <!-- Timeline Slider (shown when playing) -->
        <div
          v-if="selectedDrone"
          class="bg-surface-2 border-t border-hairline px-4 py-2 flex items-center gap-3 flex-shrink-0"
        >
          <span class="text-[10px] font-mono text-fg-mute flex-shrink-0">REPLAY</span>
          <input
            type="range"
            v-model.number="timelinePosition"
            min="0"
            max="100"
            class="flex-1 h-1 accent-accent cursor-pointer"
          />
          <span class="text-[10px] font-mono font-bold text-fg-hi w-10 text-right tabular-nums">
            {{ timelinePosition }}%
          </span>
        </div>
      </section>
    </div>

    <!-- ================================================================== -->
    <!-- FOOTER                                                             -->
    <!-- ================================================================== -->
    <footer class="bg-surface-1 border-t border-hairline-strong px-4 py-1.5 flex items-center justify-between flex-shrink-0">
      <span class="text-[10px] font-mono text-fg-dim">
        Remote ID Monitor · v1.0.0
      </span>
      <span class="text-[10px] font-mono text-fg-dim">
        {{ footerTime }}
      </span>
    </footer>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import * as L from 'leaflet'
import 'leaflet/dist/leaflet.css'

// Utils
import { timeAgo, isRecent, shortenMac, formatCoord, downloadBlob } from '@/utils/helpers'
import { setupLeafletIcons, createDroneIcon } from '@/utils/leaflet-setup'
import {
  fetchDroneDetail,
  fetchDroneTrajectory,
  fetchDroneExport,
  fetchAlerts,
  dronesToCSV,
  connectWebSocket
} from '@/utils/api'

// Store
import { useDroneStore } from '@/stores/drones'

// Icons
import DroneIcon from './components/icons/DroneIcon.vue'
import AlertIcon from './components/icons/AlertIcon.vue'
import FullScreenIcon from './components/icons/FullScreenIcon.vue'
import MinimizeIcon from './components/icons/MinimizeIcon.vue'

setupLeafletIcons()

const router = useRouter()
const store = useDroneStore()

// ---- Map Types ----
const mapTypeOptions = [
  { key: 'openstreetmap', label: 'OSM' },
  { key: 'cartodb_dark', label: 'DARK' },
  { key: 'tianditu', label: '天地图' },
  { key: 'tiandituImg', label: '卫星' }
]

// ---- State ----
const mapContainer = ref(null)
const map = ref(null)
const droneMarkers = ref({})
const timelinePosition = ref(0)
const isPlaying = ref(false)
const isFullscreen = ref(false)
const selectedDrone = ref(null)
const trajectoryLayers = ref({})
const playbackMarkers = ref({})
const selectedDroneTrajectory = ref([])
const currentMapType = ref('cartodb_dark')
const wsConnected = ref(false)
const footerTime = ref('')

let playbackInterval = null
let ws = null
let pollInterval = null

// ---- Computed ----
const compliantCount = computed(() =>
  store.activeDrones.filter(d => d.caa_compliant).length
)
const nonCompliantCount = computed(() =>
  store.activeDrones.filter(d => !d.caa_compliant).length
)
const alertSummary = computed(() => {
  const total = store.alertCount
  if (total === 0) return 'clear'
  const critical = store.alerts.filter(a => a && getAlertSeverity(a) === 'critical').length
  return critical > 0 ? `${critical} critical` : `${total} total`
})

// ---- Alert Helpers ----
const getAlertSeverity = (alert) => {
  if (!alert) return 'info'
  const type = (alert.type || '').toLowerCase()
  if (type.includes('critical') || type.includes('danger') || type.includes('emergency')) return 'critical'
  if (type.includes('warning') || type.includes('non-compliant')) return 'warning'
  return 'info'
}
const getAlertBadgeClass = (alert) => {
  const sev = getAlertSeverity(alert)
  if (sev === 'critical') return 'bg-deception/15 text-deception'
  if (sev === 'warning') return 'bg-phantom/15 text-phantom'
  return 'bg-surface-3 text-fg-mute'
}

// ---- Map Tile Config ----
const TDT_TOKEN = '66ad6f2f6216ab57401ed9907e94cd43'
const mapTypes = {
  openstreetmap: {
    url: 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png',
    attribution: '&copy; OSM',
    subdomains: []
  },
  cartodb_dark: {
    url: 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png',
    attribution: '&copy; OSM, &copy; CARTO',
    subdomains: ['a', 'b', 'c', 'd']
  },
  tianditu: {
    url: `http://t{s}.tianditu.gov.cn/vec_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=vec&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILECOL={x}&TILEROW={y}&TILEMATRIX={z}&tk=${TDT_TOKEN}`,
    subdomains: ['0', '1', '2', '3', '4', '5', '6', '7'],
    annotationUrl: `https://t{s}.tianditu.gov.cn/cva_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=cva&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILECOL={x}&TILEROW={y}&TILEMATRIX={z}&tk=${TDT_TOKEN}`,
    attribution: '&copy; 天地图'
  },
  tiandituImg: {
    url: `http://t{s}.tianditu.gov.cn/img_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=img&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILECOL={x}&TILEROW={y}&TILEMATRIX={z}&tk=${TDT_TOKEN}`,
    subdomains: ['0', '1', '2', '3', '4', '5', '6', '7'],
    annotationUrl: `https://t{s}.tianditu.gov.cn/cva_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=cva&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILECOL={x}&TILEROW={y}&TILEMATRIX={z}&tk=${TDT_TOKEN}`,
    attribution: '&copy; 天地图'
  }
}

// ---- Popup Content ----
const createPopupContent = (droneData, mac) => `
  <div class="drone-popup" data-mac="${mac}" style="font-family:var(--mono);font-size:11px;color:var(--fg-mid);min-width:180px;">
    <b style="color:var(--fg-hi);font-size:12px;">${droneData.uas_id || 'Unknown'}</b><br>
    MAC: ${mac}<br>
    Type: ${droneData.ua_type || '—'}<br>
    Alt: ${droneData.altitude?.toFixed(1) || '—'} m<br>
    <hr style="border:none;border-top:1px solid var(--hairline);margin:6px 0">
    <button class="view-details-btn"
      style="width:100%;padding:4px 8px;background:var(--accent);color:var(--void);
             border:none;font-family:var(--sans);font-size:11px;font-weight:700;
             letter-spacing:.04em;cursor:pointer;">
      VIEW DETAILS
    </button>
  </div>
`

// ---- Map Event Handler ----
const handleMapClick = (e) => {
  if (e.target.classList.contains('view-details-btn')) {
    const popupContent = e.target.closest('.drone-popup')
    if (popupContent) {
      const mac = popupContent.getAttribute('data-mac')
      if (mac) {
        map.value?.closePopup()
        router.push(`/drone/${mac}`)
      }
    }
  }
}

// ---- Map Init ----
const initMap = () => {
  try {
    if (!mapContainer.value) return
    if (map.value) map.value.remove()

    map.value = L.map(mapContainer.value, {
      zoomControl: true,
      attributionControl: true,
    }).setView([23.14287, 113.26026], 12)

    updateMapLayer()
    mapContainer.value.addEventListener('click', handleMapClick)

    setTimeout(() => map.value?.invalidateSize(), 100)
  } catch (e) {
    console.error('Map init error:', e)
  }
}

const updateMapLayer = () => {
  if (!map.value) return
  map.value.eachLayer(layer => {
    if (layer instanceof L.TileLayer) map.value.removeLayer(layer)
  })

  const config = mapTypes[currentMapType.value]
  if (currentMapType.value === 'cartodb_dark' || currentMapType.value === 'openstreetmap') {
    L.tileLayer(config.url, {
      subdomains: config.subdomains.length ? config.subdomains : undefined,
      attribution: config.attribution,
      maxZoom: 19,
    }).addTo(map.value)
  } else {
    L.tileLayer(config.url, {
      subdomains: config.subdomains,
      attribution: config.attribution,
    }).addTo(map.value)
    if (config.annotationUrl) {
      L.tileLayer(config.annotationUrl, {
        subdomains: config.subdomains,
        attribution: config.attribution,
      }).addTo(map.value)
    }
  }
}

const changeMapType = (type) => {
  currentMapType.value = type
  updateMapLayer()
}

// ---- Drone Markers ----
const updateDroneMarkers = (drones) => {
  if (!map.value) return
  const activeMacs = new Set(drones.map(d => d.mac))

  Object.keys(droneMarkers.value).forEach(mac => {
    if (!activeMacs.has(mac)) {
      map.value.removeLayer(droneMarkers.value[mac])
      delete droneMarkers.value[mac]
    }
  })

  drones.forEach(drone => {
    if (!drone.latitude || !drone.longitude) return

    const color = drone.caa_compliant ? 'green' : 'red'
    if (droneMarkers.value[drone.mac]) {
      const marker = droneMarkers.value[drone.mac]
      marker.setLatLng([drone.latitude, drone.longitude])
      marker.setIcon(createDroneIcon(color, 12))
      marker.setPopupContent(createPopupContent(drone, drone.mac))
    } else {
      const marker = L.marker([drone.latitude, drone.longitude], {
        icon: createDroneIcon(color, 12)
      }).addTo(map.value)
      marker.bindPopup(createPopupContent(drone, drone.mac))
      droneMarkers.value[drone.mac] = marker
    }
  })
}

const updateDroneMarker = (mac, droneData) => {
  if (!map.value || !droneMarkers.value[mac]) return
  const marker = droneMarkers.value[mac]
  marker.setLatLng([droneData.latitude, droneData.longitude])
  marker.setPopupContent(createPopupContent(droneData, mac))
}

// ---- Drone Selection ----
const selectDrone = async (drone) => {
  selectedDrone.value = drone
  if (map.value && drone.latitude && drone.longitude) {
    map.value.setView([drone.latitude, drone.longitude], 14)
    droneMarkers.value[drone.mac]?.openPopup()
  }
  try {
    const data = await fetchDroneDetail(drone.mac)
    selectedDrone.value = data
  } catch (e) {
    console.error('Fetch drone detail error:', e)
  }
  router.push(`/drone/${drone.mac}`)
}

// ---- Trajectories ----
const showAllTrajectories = async () => {
  if (!map.value) return
  clearTrajectories()
  for (const drone of store.activeDrones) {
    await showDroneTrajectory(drone.mac)
  }
}

const showDroneTrajectory = async (mac) => {
  if (!map.value) return
  try {
    const points = await fetchDroneTrajectory(mac)
    if (points.length > 1) {
      const latLngs = points.map(p => [p.latitude, p.longitude])
      const polyline = L.polyline(latLngs, {
        color: '#56a8ff', weight: 2, opacity: 0.6, dashArray: '4 4'
      }).addTo(map.value)

      if (latLngs.length > 0) {
        L.circleMarker(latLngs[0], {
          radius: 4, color: '#19d27a', fillColor: '#19d27a', fillOpacity: 1, weight: 2
        }).addTo(map.value)
        L.circleMarker(latLngs[latLngs.length - 1], {
          radius: 4, color: '#ff4d6d', fillColor: '#ff4d6d', fillOpacity: 1, weight: 2
        }).addTo(map.value)
      }
      trajectoryLayers.value[mac] = polyline
      const bounds = L.latLngBounds(latLngs)
      map.value.fitBounds(bounds, { padding: [40, 40] })
    }
  } catch (e) {
    console.error(`Trajectory error for ${mac}:`, e)
  }
}

const clearTrajectories = () => {
  if (!map.value) return
  Object.values(trajectoryLayers.value).forEach(layer => {
    map.value.hasLayer(layer) && map.value.removeLayer(layer)
  })
  trajectoryLayers.value = {}
  Object.values(playbackMarkers.value).forEach(marker => {
    map.value.hasLayer(marker) && map.value.removeLayer(marker)
  })
  playbackMarkers.value = {}
}

// ---- Playback ----
const loadDroneTrajectory = async (mac) => {
  try {
    selectedDroneTrajectory.value = await fetchDroneTrajectory(mac)
  } catch (e) {
    console.error('Load trajectory error:', e)
  }
}

const updatePlaybackMarker = (position) => {
  if (!map.value || !selectedDrone.value) return
  if (playbackMarkers.value[selectedDrone.value.mac]) {
    map.value.removeLayer(playbackMarkers.value[selectedDrone.value.mac])
  }
  const marker = L.marker([position.latitude, position.longitude], {
    icon: createDroneIcon('blue', 16, true)
  }).addTo(map.value)
  marker.bindPopup(`Replay: ${formatCoord(position.latitude)}, ${formatCoord(position.longitude)}`)
  playbackMarkers.value[selectedDrone.value.mac] = marker
  map.value.setView([position.latitude, position.longitude], 14)
}

const togglePlayback = () => {
  if (!map.value || !selectedDrone.value) return
  isPlaying.value = !isPlaying.value
  isPlaying.value ? startPlayback() : stopPlayback()
}

const stopPlayback = () => {
  isPlaying.value = false
  if (playbackInterval) {
    clearInterval(playbackInterval)
    playbackInterval = null
  }
}

const startPlayback = async () => {
  if (!map.value || !isPlaying.value || !selectedDrone.value) return
  if (!selectedDroneTrajectory.value.length) {
    await loadDroneTrajectory(selectedDrone.value.mac)
  }
  if (!selectedDroneTrajectory.value.length) return
  if (playbackInterval) clearInterval(playbackInterval)

  const totalPoints = selectedDroneTrajectory.value.length
  let currentIndex = Math.floor((timelinePosition.value / 100) * totalPoints)

  playbackInterval = setInterval(() => {
    if (!isPlaying.value || !map.value) { stopPlayback(); return }
    if (currentIndex < totalPoints) {
      updatePlaybackMarker(selectedDroneTrajectory.value[currentIndex])
      timelinePosition.value = (currentIndex / totalPoints) * 100
      currentIndex++
    } else {
      stopPlayback()
    }
  }, 100)
}

// ---- Export ----
const exportData = async () => {
  try {
    if (!store.activeDrones.length) return
    const macs = selectedDrone.value
      ? [selectedDrone.value.mac]
      : store.activeDrones.map(d => d.mac)

    const allData = []
    for (const mac of macs) {
      try { allData.push(await fetchDroneExport(mac)) } catch (e) { /* skip */ }
    }

    const csvContent = dronesToCSV(allData)
    const prefix = selectedDrone.value
      ? `drone_${selectedDrone.value.mac.replace(/:/g, '_')}`
      : 'all_drones'
    const filename = `${prefix}_export_${new Date().toISOString().split('T')[0]}.csv`
    downloadBlob(new Blob([csvContent], { type: 'text/csv;charset=utf-8;' }), filename)
  } catch (e) {
    console.error('Export error:', e)
  }
}

// ---- Fullscreen ----
const toggleFullscreen = () => {
  if (!document.fullscreenElement) {
    document.documentElement.requestFullscreen()
    isFullscreen.value = true
  } else {
    document.exitFullscreen()
    isFullscreen.value = false
  }
}

// ---- WebSocket ----
const initWebSocket = () => {
  try {
    ws = connectWebSocket({
      onDroneUpdate: (mac, data) => {
        wsConnected.value = true
        updateDroneMarker(mac, data)
      }
    })
    // Monitor connection
    if (ws) {
      ws.addEventListener('open', () => { wsConnected.value = true })
      ws.addEventListener('close', () => { wsConnected.value = false })
      ws.addEventListener('error', () => { wsConnected.value = false })
    }
  } catch (e) {
    console.error('WebSocket init error:', e)
    wsConnected.value = false
  }
}

// ---- Data Polling ----
const refreshData = async () => {
  try {
    const drones = await store.loadActiveDrones()
    store.cleanStaleDrones()
    updateDroneMarkers(drones)
    await store.loadAlerts()
    footerTime.value = new Date().toLocaleTimeString()
  } catch (e) {
    console.error('Refresh error:', e)
  }
}

// ---- Footer Clock ----
let footerInterval = null

// ---- Lifecycle ----
onMounted(() => {
  nextTick(() => {
    initMap()
    initWebSocket()
  })
  refreshData()
  pollInterval = setInterval(refreshData, 5000)
  footerInterval = setInterval(() => {
    footerTime.value = new Date().toLocaleTimeString()
  }, 1000)
})

onUnmounted(() => {
  if (playbackInterval) { clearInterval(playbackInterval); playbackInterval = null }
  if (pollInterval) { clearInterval(pollInterval); pollInterval = null }
  if (footerInterval) { clearInterval(footerInterval); footerInterval = null }
  if (ws) { ws.close(); ws = null }
  if (map.value) {
    map.value.closePopup()
    map.value.eachLayer(layer => map.value.removeLayer(layer))
    if (mapContainer.value) mapContainer.value.removeEventListener('click', handleMapClick)
    map.value.remove()
    map.value = null
  }
})
</script>

<style scoped>
/* =========================================================================
   Component-scoped styles (dark theme)
   ========================================================================= */

/* ---- Summary Cards ---- */
.card-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  background: #131820;
  border: 1px solid #232b35;
  border-left-width: 3px;
  border-left-color: #232b35;
  padding: 6px 10px;
  min-width: 0;
}
.card-label {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}
.card-num {
  font-family: ui-monospace, 'JetBrains Mono', SFMono-Regular, Menlo, monospace;
  font-size: 22px;
  font-weight: 700;
  line-height: 1;
  color: #e8eef7;
  font-variant-numeric: tabular-nums;
}

/* ---- Pane Head ---- */
.pane-head {
  background: #131820;
  border-bottom: 1px solid #1a2027;
  padding: 6px 12px;
  font-family: ui-monospace, 'JetBrains Mono', SFMono-Regular, Menlo, monospace;
  font-size: 10px;
  color: #687586;
  letter-spacing: 0.04em;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}
.pane-head b {
  color: #e8eef7;
}

/* ---- Drone List Items ---- */
.drone-item {
  padding: 8px 10px;
  background: #131820;
  border: 1px solid #1a2027;
  border-left: 3px solid #232b35;
  cursor: pointer;
  transition: background 150ms ease, border-left-color 150ms ease;
}
.drone-item:hover {
  background: #181f29;
  border-left-color: #56a8ff;
}
.drone-item[data-status="compliant"] {
  border-left-color: #19d27a;
}
.drone-item[data-status="noncompliant"] {
  border-left-color: #ff4d6d;
}

/* ---- Alert Items ---- */
.alert-item {
  padding: 8px 10px;
  background: #131820;
  border: 1px solid #1a2027;
  border-left: 3px solid #232b35;
}
.alert-item[data-severity="critical"] {
  border-left-color: #ff4d6d;
}
.alert-item[data-severity="warning"] {
  border-left-color: #ff9633;
}
.alert-item[data-severity="info"] {
  border-left-color: #56a8ff;
}

/* ---- Map ---- */
#main-map {
  min-height: 0;
}
#main-map :deep(.leaflet-container) {
  height: 100%;
  width: 100%;
  background: #0e1117 !important;
}

/* ---- Range Slider ---- */
input[type="range"] {
  -webkit-appearance: none;
  appearance: none;
  height: 4px;
  background: #232b35;
  border-radius: 2px;
  outline: none;
}
input[type="range"]::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: #56a8ff;
  cursor: pointer;
  border: 2px solid #0e1117;
  box-shadow: 0 0 6px rgba(86, 168, 255, 0.4);
}

/* ---- Status Dot ---- */
.status-dot {
  display: inline-block;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #424c5a;
  flex-shrink: 0;
}
.status-dot.live {
  background: #19d27a;
  animation: dotPulse 1.6s ease-in-out infinite;
}
.status-dot.error {
  background: #ff4d6d;
  animation: none;
}

@keyframes dotPulse {
  0%, 100% { opacity: 1; }
  50%      { opacity: .4; }
}
</style>
