<template>
  <div class="app-layout">
    <AppHeader />
    <div class="main-content">
      <AppSidebar />
      <main class="content-area">
        <!-- 路由出口：具体页面渲染在这里 -->
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </main>
    </div>
    <GlobalNotification />
  </div>
</template>

<script setup>
import AppHeader from './AppHeader.vue';
import AppSidebar from './AppSidebar.vue';
import GlobalNotification from './GlobalNotification.vue';
</script>

<style scoped>
.app-layout { display: flex; flex-direction: column; height: 100vh; }
.main-content { display: flex; flex: 1; overflow: hidden; }
.content-area { flex: 1; overflow-y: auto; padding: 20px; }
</style>