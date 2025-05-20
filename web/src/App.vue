<template>
  <a-layout style="min-height: 100vh">
    <!-- 在大屏幕上显示的侧边菜单 -->
    <a-layout-sider 
      v-model:collapsed="collapsed"
      width="240" 
      theme="light" 
      style="border-right: 1px solid #f0f0f0; box-shadow: 0 0 10px rgba(0,0,0,0.05);" 
      breakpoint="lg" 
      collapsible 
      :collapsed-width="80"
      v-if="!isMobile"
    >
      <div style="padding: 24px 16px; text-align: center; font-weight: bold; font-size: 20px; color: #1890ff; display: flex; align-items: center; justify-content: center; gap: 8px;">
        <DownloadOutlined style="font-size: 24px" />
        <span v-if="!collapsed">M3U8 下载器</span>
      </div>
      <a-menu mode="inline" :selected-keys="[activeMenu]" style="border-right: none">
        <a-menu-item key="download" @click="navigateTo('/')" class="menu-item">
          <template #icon><DownloadOutlined /></template>
          <span>下载管理</span>
        </a-menu-item>
        <a-menu-item key="setting" @click="navigateTo('/settings')" class="menu-item">
          <template #icon><SettingOutlined /></template>
          <span>配置设置</span>
        </a-menu-item>
      </a-menu>
    </a-layout-sider>
    <a-layout>
      <!-- 在小屏幕上显示的顶部菜单 -->
      <a-layout-header
        v-if="isMobile"
        style="background: white; padding: 0 16px; box-shadow: 0 1px 4px rgba(0,0,0,0.1); display: flex; align-items: center; justify-content: space-between;"
      >
        <div style="font-weight: bold; font-size: 16px; color: #1890ff; display: flex; align-items: center; gap: 8px;">
          <DownloadOutlined />
          <span>M3U8 下载器</span>
        </div>
        <a-menu mode="horizontal" :selected-keys="[activeMenu]" style="border-bottom: none; flex: 1; justify-content: flex-end;">
          <a-menu-item key="download" @click="navigateTo('/')" class="menu-item-horizontal">
            <template #icon><DownloadOutlined /></template>
            <span>下载管理</span>
          </a-menu-item>
          <a-menu-item key="setting" @click="navigateTo('/settings')" class="menu-item-horizontal">
            <template #icon><SettingOutlined /></template>
            <span>配置设置</span>
          </a-menu-item>
        </a-menu>
      </a-layout-header>
      <a-layout-content :style="{ padding: '24px', background: '#f7f9fa', paddingTop: isMobile ? '12px' : '24px' }">
        <router-view />
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>

<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { DownloadOutlined, SettingOutlined } from '@ant-design/icons-vue'

const router = useRouter()
const route = useRoute()
const activeMenu = ref('download')
const collapsed = ref(false)
const isMobile = ref(false)

// 监听窗口大小变化，更新isMobile状态
const checkScreenSize = () => {
  isMobile.value = window.innerWidth < 768
}

onMounted(() => {
  checkScreenSize()
  window.addEventListener('resize', checkScreenSize)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkScreenSize)
})

// 根据当前路由设置活动菜单项
watch(() => route.path, (path) => {
  if (path === '/') {
    activeMenu.value = 'download'
  } else if (path === '/settings') {
    activeMenu.value = 'setting'
  }
}, { immediate: true })

// 导航函数
const navigateTo = (path) => {
  router.push(path)
}
</script>

<style>
body {
  margin: 0;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
  background-color: #f7f9fa;
}

#app {
  height: 100%;
}

.menu-item {
  transition: all 0.3s ease;
  margin: 4px 0;
}

.menu-item:hover {
  background-color: #e6f7ff;
}

.menu-item-horizontal {
  transition: all 0.3s ease;
}

.menu-item-horizontal:hover {
  color: #1890ff;
}

:deep(.ant-layout-sider-children) {
  display: flex;
  flex-direction: column;
}

:deep(.ant-menu-item-selected) {
  background-color: #e6f7ff !important;
  font-weight: 500;
}

:deep(.ant-menu-item::after) {
  border-right: 3px solid #1890ff;
}

@media (max-width: 768px) {
  :deep(.ant-layout-content) {
    padding: 12px 8px !important;
  }
  
  :deep(.ant-menu-horizontal) {
    line-height: 46px;
  }
  
  :deep(.ant-layout-header) {
    height: 56px;
    line-height: 56px;
  }
}
</style> 