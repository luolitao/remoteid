<!-- Drones list view — tar1090 table style -->
<template>
  <div class="h-screen flex flex-col" style="background: var(--BGCOLOR1); font-size: var(--FS2);">
    <header class="flex items-center justify-between px-3 py-2 flex-shrink-0" style="border-bottom: 1px solid var(--BGCOLOR2);">
      <div class="flex items-center gap-3">
        <RouterLink to="/" class="text-sm" style="color: var(--ACCENT);">← Back</RouterLink>
        <span class="identMedium">Active Drones ({{ store.activeDrones.length }})</span>
      </div>
    </header>

    <div class="flex-1 overflow-auto">
      <table class="w-full text-xs" style="font-size: var(--FS2);">
        <thead>
          <tr class="aircraft_table_header sticky top-0 z-10" style="background: var(--ACCENT); color: #FFF;">
            <th class="text-left px-3 py-1.5 font-normal">ID</th>
            <th class="text-left px-3 py-1.5 font-normal">Op. ID</th>
            <th class="text-left px-3 py-1.5 font-normal">MAC</th>
            <th class="text-left px-3 py-1.5 font-normal">Type</th>
            <th class="text-right px-3 py-1.5 font-normal">Lat</th>
            <th class="text-right px-3 py-1.5 font-normal">Lon</th>
            <th class="text-right px-3 py-1.5 font-normal">Alt</th>
            <th class="text-right px-3 py-1.5 font-normal">Last Seen</th>
            <th class="text-center px-3 py-1.5 font-normal">Compliance</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="drone in store.activeDrones"
            :key="drone.mac"
            class="plane_table_row cursor-pointer hover:bg-blue-50 border-b"
            style="border-color: var(--BGCOLOR2);"
            @click="$router.push(`/drone/${drone.mac}`)"
          >
            <td class="px-3 py-1 font-bold" style="color: var(--TXTCOLOR1);">{{ drone.uas_id || '-' }}</td>
            <td class="px-3 py-1 font-mono text-xs">{{ drone.operator_id || '-' }}</td>
            <td class="px-3 py-1 font-mono text-xs">{{ shortenMac(drone.mac, true) }}</td>
            <td class="px-3 py-1">{{ drone.ua_type || '-' }}</td>
            <td class="px-3 py-1 text-right font-mono">{{ formatCoord(drone.latitude) }}</td>
            <td class="px-3 py-1 text-right font-mono">{{ formatCoord(drone.longitude) }}</td>
            <td class="px-3 py-1 text-right font-mono">{{ drone.altitude ? drone.altitude.toFixed(1)+'m' : '-' }}</td>
            <td class="px-3 py-1 text-right">{{ timeAgo(drone.last_seen) }}</td>

          </tr>
          <tr v-if="store.activeDrones.length === 0">
            <td colspan="9" class="text-center py-8" style="color: var(--TXTCOLOR2);">No active drones</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import { RouterLink } from 'vue-router'
import { useDroneStore } from '@/stores/drones'
import { timeAgo, formatCoord, shortenMac } from '@/utils/helpers'

const store = useDroneStore()
</script>
