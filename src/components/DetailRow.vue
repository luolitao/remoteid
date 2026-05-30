<template>
  <div class="flex justify-between items-center py-2 border-b border-hairline last:border-b-0">
    <span class="text-xs text-fg-mute font-mono tracking-wider uppercase">{{ label }}</span>
    <div class="flex items-center">
      <span v-if="!badge" class="text-xs text-fg-hi font-mono font-bold tabular-nums">{{ value }}</span>
      <span
        v-else
        :class="badge"
        class="px-2 py-0.5 rounded text-[10px] font-mono font-bold tracking-wider"
      >
        {{ value }}
      </span>
    </div>
  </div>
</template>

<script setup>
defineProps({
  label: {
    type: String,
    required: true
  },
  value: {
    type: String,
    default: ''
  },
  badge: {
    type: String,
    default: null
  }
})
</script>
