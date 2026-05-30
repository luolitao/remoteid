<!-- tar1090 infoBlock style: infoHeading (left) + infoData (right) -->
<template>
  <div>
    <div class="infoHeading">{{ label }}:</div>
    <div class="infoData" :class="{ 'font-mono': mono }">
      <span v-if="!badge">{{ value }}</span>
      <span v-else :class="badge" class="px-1.5 py-0.5 rounded text-xs font-bold">{{ value }}</span>
    </div>
  </div>
</template>

<script setup>
defineProps({
  label: { type: String, required: true },
  value: { type: String, default: '' },
  badge: { type: String, default: null },
  mono: { type: Boolean, default: false },
})
</script>
