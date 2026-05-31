import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/',
    name: 'Home',
    component: () => import('../views/Home.vue')
  },
  // nginx try_files 可能重定向到 /index.html，需要 redirect 回 /
  {
    path: '/index.html',
    redirect: '/'
  },
  {
    path: '/map',
    name: 'Map',
    component: () => import('../views/Map.vue')
  },
  {
    path: '/drones',
    name: 'Drones',
    component: () => import('../views/Drones.vue')
  },
  {
    path: '/alerts',
    name: 'Alerts',
    component: () => import('../views/Alerts.vue')
  },
  {
    path: '/drone/:mac',
    name: 'DroneDetail',
    component: () => import('../views/DroneDetail.vue'),
    props: true
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

export default router