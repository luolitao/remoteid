<template>
  <div class="h-screen flex flex-col bg-void overflow-hidden">
    <!-- Header -->
    <header class="bg-surface-1 border-b border-hairline-strong px-4 py-2 flex items-center justify-between flex-shrink-0">
      <div class="flex items-center gap-3 min-w-0">
        <RouterLink to="/" class="text-fg-mute hover:text-accent transition-colors">
          <ArrowLeftIcon class="w-5 h-5" />
        </RouterLink>
        <div class="min-w-0">
          <h1 class="text-base font-extrabold text-fg-hi truncate tracking-tight">
            {{ drone?.uas_id || 'Unknown Drone' }}
          </h1>
          <p class="text-[11px] text-fg-mute font-mono">
            MAC: {{ shortenMac(drone?.mac, true) }}
          </p>
        </div>
      </div>
      <div class="flex items-center gap-2 flex-shrink-0">
        <!-- Compliance Badge -->
        <span
          class="text-[10px] px-2 py-1 rounded font-mono font-bold tracking-wider"
          :class="drone?.caa_compliant
            ? 'bg-confirmed/15 text-confirmed'
            : 'bg-deception/15 text-deception'"
        >
          {{ complianceStatus }}
        </span>
        <button
          @click="refreshData"
          class="p-1.5 text-fg-mute hover:text-accent hover:bg-surface-2 rounded transition-colors"
          title="Refresh"
        >
          <RefreshIcon class="w-4 h-4" />
        </button>
        <button
          @click="showTrajectory = true"
          class="px-3 py-1.5 text-xs font-bold tracking-wider bg-accent text-void border border-accent
                 hover:-translate-y-px active:translate-y-0 transition-transform cursor-pointer"
        >
          <MapIcon class="w-3.5 h-3.5 inline mr-1" /> TRAJECTORY
        </button>
      </div>
    </header>

    <!-- Main Content -->
    <div class="flex-1 overflow-y-auto p-4">
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4 max-w-7xl mx-auto">
        <!-- Left Column: Position + Map + History -->
        <div class="lg:col-span-2 space-y-4">
          <!-- Position Card -->
          <section class="bg-surface-1 border border-hairline rounded-lg overflow-hidden">
            <div class="pane-head">
              <div class="flex items-center gap-2">
                <MapPinIcon class="w-4 h-4 text-deception" />
                <span>REAL-TIME POSITION</span>
              </div>
              <span class="text-fg-dim">{{ timeAgo(drone?.last_seen) }}</span>
            </div>
            <div class="p-4 grid grid-cols-2 md:grid-cols-4 gap-3">
              <div class="stat-cell">
                <span class="stat-label">LATITUDE</span>
                <span class="stat-value text-accent">{{ formatCoord(drone?.latitude) }}</span>
              </div>
              <div class="stat-cell">
                <span class="stat-label">LONGITUDE</span>
                <span class="stat-value text-accent">{{ formatCoord(drone?.longitude) }}</span>
              </div>
              <div class="stat-cell">
                <span class="stat-label">ALTITUDE</span>
                <span class="stat-value text-confirmed">
                  {{ drone?.altitude ? `${drone.altitude.toFixed(1)} m` : '—' }}
                </span>
              </div>
              <div class="stat-cell">
                <span class="stat-label">LAST UPDATED</span>
                <span class="stat-value text-suspect">{{ timeAgo(drone?.last_seen) }}</span>
              </div>
            </div>
          </section>

          <!-- Map Card -->
          <section class="bg-surface-1 border border-hairline rounded-lg overflow-hidden flex flex-col">
            <div class="pane-head">
              <div class="flex items-center gap-2">
                <MapIcon class="w-4 h-4 text-accent" />
                <span>CURRENT LOCATION</span>
              </div>
            </div>
            <div class="h-80 w-full">
              <DroneMap :drone="drone" :show-trajectory="showTrajectory" />
            </div>
          </section>

          <!-- Position History Table -->
          <section v-if="positionHistory.length > 0" class="bg-surface-1 border border-hairline rounded-lg overflow-hidden">
            <div class="pane-head">
              <div class="flex items-center gap-2">
                <ClockIcon class="w-4 h-4 text-suspect" />
                <span>POSITION HISTORY</span>
              </div>
              <b>Last {{ positionHistory.length }}</b>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full">
                <thead>
                  <tr class="bg-surface-2">
                    <th class="th-cell">TIME</th>
                    <th class="th-cell">LATITUDE</th>
                    <th class="th-cell">LONGITUDE</th>
                    <th class="th-cell">ALTITUDE</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="(pos, index) in positionHistory"
                    :key="index"
                    class="border-b border-hairline hover:bg-surface-2 transition-colors"
                  >
                    <td class="td-cell text-fg-mute font-mono text-xs">{{ formatTime(pos.timestamp) }}</td>
                    <td class="td-cell text-accent font-mono text-xs">{{ formatCoord(pos.latitude) }}</td>
                    <td class="td-cell text-accent font-mono text-xs">{{ formatCoord(pos.longitude) }}</td>
                    <td class="td-cell text-confirmed font-mono text-xs">
                      {{ pos.altitude ? `${pos.altitude.toFixed(1)} m` : '—' }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>

        <!-- Right Column: Info Panels -->
        <div class="space-y-4">
          <!-- System Information -->
          <section class="bg-surface-1 border border-hairline rounded-lg overflow-hidden">
            <div class="pane-head">
              <div class="flex items-center gap-2">
                <ChipIcon class="w-4 h-4 text-fg-mute" />
                <span>SYSTEM INFO</span>
              </div>
            </div>
            <div class="p-4 space-y-2">
              <DetailRow label="Standard" :value="drone?.standard || 'Unknown'" />
              <DetailRow label="Aircraft ID" :value="drone?.uas_id || 'N/A'" />
              <DetailRow label="Aircraft Type" :value="drone?.ua_type || 'N/A'" />
              <DetailRow label="Compliance" :value="complianceStatus" :badge="complianceBadge" />
              <DetailRow label="First Seen" :value="formatTime(drone?.first_seen)" />
              <DetailRow label="Signal Strength" :value="drone?.signal_strength || 'N/A'" />
            </div>
          </section>

          <!-- Operator Information -->
          <section v-if="drone?.operator_latitude" class="bg-surface-1 border border-hairline rounded-lg overflow-hidden">
            <div class="pane-head">
              <div class="flex items-center gap-2">
                <UserIcon class="w-4 h-4 text-accent" />
                <span>OPERATOR INFO</span>
              </div>
            </div>
            <div class="p-4 space-y-2">
              <DetailRow
                label="Operator Position"
                :value="`${formatCoord(drone.operator_latitude)}, ${formatCoord(drone.operator_longitude)}`"
              />
              <DetailRow v-if="drone.area_radius_m" label="Operation Radius" :value="`${drone.area_radius_m} m`" />
              <DetailRow v-if="drone.classification_region" label="Region" :value="drone.classification_region" />
            </div>
          </section>

          <!-- Alert History -->
          <section v-if="alertHistory.length > 0" class="bg-surface-1 border border-hairline rounded-lg overflow-hidden">
            <div class="pane-head">
              <div class="flex items-center gap-2">
                <BellIcon class="w-4 h-4 text-deception" />
                <span>ALERT HISTORY</span>
              </div>
              <b class="text-deception">{{ alertHistory.length }}</b>
            </div>
            <div class="p-3 space-y-2">
              <div
                v-for="(alert, index) in alertHistory"
                :key="index"
                class="bg-surface-2 border border-hairline border-l-3 border-l-deception p-3 rounded-r"
              >
                <div class="flex justify-between items-start">
                  <div>
                    <p class="text-xs font-bold text-deception">{{ alert.type }}</p>
                    <p class="text-[11px] text-fg-mid mt-1">{{ alert.message }}</p>
                  </div>
                  <span class="text-[10px] text-fg-dim font-mono bg-surface-3 px-2 py-0.5 rounded">
                    {{ formatTime(alert.timestamp) }}
                  </span>
                </div>
              </div>
            </div>
          </section>

          <!-- Actions -->
          <section class="bg-surface-1 border border-hairline rounded-lg overflow-hidden">
            <div class="pane-head">
              <div class="flex items-center gap-2">
                <CogIcon class="w-4 h-4 text-fg-mute" />
                <span>ACTIONS</span>
              </div>
            </div>
            <div class="p-4 space-y-2">
              <button
                @click="exportData"
                class="w-full flex items-center justify-center gap-2 px-4 py-2 text-xs font-bold tracking-wider
                       bg-transparent text-fg-mid border border-hairline-strong
                       hover:text-fg-hi hover:border-accent transition-all duration-150 cursor-pointer"
              >
                <DownloadIcon class="w-3.5 h-3.5" /> EXPORT CSV
              </button>
              <button
                @click="startRecording"
                class="w-full flex items-center justify-center gap-2 px-4 py-2 text-xs font-bold tracking-wider
                       bg-confirmed text-void border border-confirmed
                       hover:-translate-y-px active:translate-y-0 transition-transform cursor-pointer"
              >
                <PlayIcon class="w-3.5 h-3.5" /> START RECORDING
              </button>
              <button
                @click="addToWatchlist"
                class="w-full flex items-center justify-center gap-2 px-4 py-2 text-xs font-bold tracking-wider
                       bg-accent text-void border border-accent
                       hover:-translate-y-px active:translate-y-0 transition-transform cursor-pointer"
              >
                <StarIcon class="w-3.5 h-3.5" /> ADD TO WATCHLIST
              </button>
            </div>
          </section>
        </div>
      </div>
    </div>

    <!-- Trajectory Modal -->
    <Teleport to="body">
      <div
        v-if="showTrajectory"
        class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4"
        @click.self="showTrajectory = false"
      >
        <div class="bg-surface-1 border border-hairline-strong rounded-lg shadow-2xl max-w-4xl w-full max-h-[90vh] overflow-hidden">
          <div class="pane-head">
            <h2 class="text-sm font-extrabold text-fg-hi">FLIGHT TRAJECTORY</h2>
            <button @click="showTrajectory = false" class="text-fg-mute hover:text-fg-hi transition-colors">
              <XIcon class="w-5 h-5" />
            </button>
          </div>
          <div class="h-[500px]">
            <TrajectoryMap :positions="positionHistory" />
          </div>
          <div class="bg-surface-2 border-t border-hairline px-4 py-2 flex justify-end">
            <button
              @click="showTrajectory = false"
              class="px-4 py-1.5 text-xs font-bold tracking-wider bg-accent text-void border border-accent
                     hover:-translate-y-px active:translate-y-0 transition-transform cursor-pointer"
            >
              CLOSE
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { timeAgo, formatTime, formatCoord, shortenMac, downloadBlob } from '@/utils/helpers'
import { fetchDroneDetail, fetchDroneExport, dronesToCSV } from '@/utils/api'

import MapPinIcon from '@/components/icons/MapPinIcon.vue'
import MapIcon from '@/components/icons/MapIcon.vue'
import ClockIcon from '@/components/icons/ClockIcon.vue'
import ChipIcon from '@/components/icons/ChipIcon.vue'
import UserIcon from '@/components/icons/UserIcon.vue'
import BellIcon from '@/components/icons/BellIcon.vue'
import CogIcon from '@/components/icons/CogIcon.vue'
import DownloadIcon from '@/components/icons/DownloadIcon.vue'
import PlayIcon from '@/components/icons/PlayIcon.vue'
import StarIcon from '@/components/icons/StarIcon.vue'
import ArrowLeftIcon from '@/components/icons/ArrowLeftIcon.vue'
import RefreshIcon from '@/components/icons/RefreshIcon.vue'
import XIcon from '@/components/icons/XIcon.vue'
import DroneMap from '@/components/DroneMap.vue'
import TrajectoryMap from '@/components/TrajectoryMap.vue'
import DetailRow from '@/components/DetailRow.vue'

const route = useRoute()
const mac = route.params.mac
const drone = ref(null)
const positionHistory = ref([])
const alertHistory = ref([])
const showTrajectory = ref(false)
const loading = ref(true)

let pollInterval = null

const complianceStatus = computed(() =>
  drone.value?.caa_compliant ? 'CAA Compliant' : 'Non-compliant'
)
const complianceBadge = computed(() =>
  drone.value?.caa_compliant
    ? 'bg-confirmed/15 text-confirmed'
    : 'bg-deception/15 text-deception'
)

const fetchDetails = async () => {
  try {
    loading.value = true
    const data = await fetchDroneDetail(mac)
    drone.value = data
    positionHistory.value = data.position_history || []
    alertHistory.value = data.alert_history || []
  } catch (error) {
    console.error('Error fetching drone details:', error)
  } finally {
    loading.value = false
  }
}

const refreshData = () => fetchDetails()
const startRecording = () => console.log('Starting recording for drone:', mac)
const addToWatchlist = () => console.log('Adding to watchlist:', mac)

const exportData = async () => {
  try {
    const droneData = await fetchDroneExport(mac)
    const csvContent = dronesToCSV([droneData])
    const filename = `drone_${mac.replace(/:/g, '_')}_export_${new Date().toISOString().split('T')[0]}.csv`
    downloadBlob(new Blob([csvContent], { type: 'text/csv;charset=utf-8;' }), filename)
  } catch (error) {
    console.error('Export failed:', error)
  }
}

onMounted(() => {
  fetchDetails()
  pollInterval = setInterval(fetchDetails, 5000)
})

onUnmounted(() => {
  if (pollInterval) clearInterval(pollInterval)
})
</script>

<style scoped>
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

/* ---- Stat Cells ---- */
.stat-cell {
  background: #131820;
  border: 1px solid #1a2027;
  padding: 10px 12px;
  border-left: 3px solid #232b35;
}
.stat-label {
  display: block;
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: #687586;
  margin-bottom: 4px;
}
.stat-value {
  font-family: ui-monospace, 'JetBrains Mono', SFMono-Regular, Menlo, monospace;
  font-size: 15px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

/* ---- Table ---- */
table {
  border-collapse: collapse;
  width: 100%;
}
.th-cell {
  text-align: left;
  padding: 6px 10px;
  font-family: ui-monospace, 'JetBrains Mono', SFMono-Regular, Menlo, monospace;
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: #687586;
}
.td-cell {
  padding: 6px 10px;
  font-variant-numeric: tabular-nums;
}

/* ---- Border left 3px utility ---- */
.border-l-3 {
  border-left-width: 3px;
}

/* ---- Map container ---- */
:deep(.map-container) {
  height: 100%;
  width: 100%;
  min-height: 300px;
}
</style>
