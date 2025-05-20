import { createRouter, createWebHistory } from 'vue-router'
import DownloadManager from '../views/DownloadManager.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'download',
      component: DownloadManager,
      meta: { title: '下载管理' }
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('../views/Settings.vue'),
      meta: { title: '配置设置' }
    }
  ]
})

router.beforeEach((to, from, next) => {
  document.title = `${to.meta.title || 'M3U8下载器'}`
  next()
})

export default router 