<!-- Alerts view — tar1090 table style -->
<template>
  <div class="h-screen flex flex-col" style="background: var(--BGCOLOR1); font-size: var(--FS2);">
    <header class="flex items-center justify-between px-3 py-2 flex-shrink-0" style="border-bottom: 1px solid var(--BGCOLOR2);">
      <div class="flex items-center gap-3">
        <RouterLink to="/" class="text-sm" style="color: var(--ACCENT);">← Back</RouterLink>
        <span class="identMedium">System Alerts ({{ store.alerts.filter(a => a).length }})</span>
      </div>
    </header>

    <div class="flex-1 overflow-auto">
      <table class="w-full text-xs" style="font-size: var(--FS2);">
        <thead>
          <tr class="aircraft_table_header sticky top-0 z-10" style="background: var(--ACCENT); color: #FFF;">
            <th class="text-left px-3 py-1.5 font-normal">Type</th>
            <th class="text-left px-3 py-1.5 font-normal">Message</th>
            <th class="text-right px-3 py-1.5 font-normal">Timestamp</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(alert, i) in store.alerts.filter(a => a)"
            :key="i"
            class="border-b"
            style="border-color: var(--BGCOLOR2);"
          >
            <td class="px-3 py-1.5">
              <span
                class="px-1.5 py-0.5 rounded text-xs font-bold"
                :style="{
                  background: getAlertBg(alert.type),
                  color: getAlertColor(alert.type)
                }"
              >
                {{ alert.type || 'Unknown' }}
              </span>
            </td>
            <td class="px-3 py-1.5" style="color: var(--TXTCOLOR2);">{{ alert.message || '-' }}</td>
            <td class="px-3 py-1.5 text-right opacity-60">{{ alert.timestamp || '-' }}</td>
          </tr>
          <tr v-if="!store.alerts.filter(a => a).length">
            <td colspan="3" class="text-center py-8" style="color: var(--TXTCOLOR2);">No alerts</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import { RouterLink } from 'vue-router'
import { useDroneStore } from '@/stores/drones'

const store = useDroneStore()

const getAlertColor = (type) => {
  if (!type) return '#666'
  const t = type.toLowerCase()
  if (t.includes('non-compliant') || t.includes('violation')) return '#991b1b'
  if (t.includes('warn')) return '#92400e'
  return '#1e40af'
}

const getAlertBg = (type) => {
  if (!type) return '#f3f4f6'
  const t = type.toLowerCase()
  if (t.includes('non-compliant') || t.includes('violation')) return '#fee2e2'
  if (t.includes('warn')) return '#fef3c7'
  return '#dbeafe'
}
</script>
