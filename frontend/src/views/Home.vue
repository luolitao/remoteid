<!-- Home view — redirects to main App layout -->
<template>
  <div class="h-screen flex flex-col items-center justify-center" style="background: var(--BGCOLOR1);">
    <div class="text-center">
      <h1 class="identLarge mb-2">Remote ID Monitor</h1>
      <p style="color: var(--TXTCOLOR2); font-size: var(--FS2); margin-bottom: 16px;">
        Real-time drone tracking and monitoring
      </p>
      <RouterLink
        to="/map"
        class="sidebarButton inline-block px-4 py-2 text-sm"
        style="text-decoration: none;"
      >
        Open Live Map
      </RouterLink>
    </div>
  </div>
</template>

<script setup>
import { RouterLink } from 'vue-router'
</script>
