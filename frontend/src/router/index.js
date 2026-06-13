import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  // nginx try_files 可能重定向到 /index.html，需要 redirect 回 /
  {
    path: '/index.html',
    redirect: '/',
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

export default router
