<template>
  <div class="download-manager">
    <a-spin :spinning="store.initialLoading" tip="加载中..." class="full-page-loading">
      <a-card class="main-card">
        <!-- 页头部分 -->
        <div class="header fade-in">
          <div class="title-section">
            <a-typography-title :level="3" style="margin: 0; color: #1890ff; font-size: 20px;">
              <DownloadOutlined class="title-icon" /> M3U8 下载管理
            </a-typography-title>
            <a-typography-paragraph style="margin: 4px 0 0 0; color: #666; font-size: 13px;">
              轻松下载和管理M3U8视频，支持MP4转换
            </a-typography-paragraph>
          </div>
          <div class="action-buttons">
            <a-tag v-if="speedLimit > 0" color="orange" class="global-limit-tag">
              <ThunderboltOutlined /> 全局限速: {{ formatLimit(speedLimit) }}
            </a-tag>
            <a-button @click="refreshTasks" :loading="store.refreshing" type="default" class="btn-with-effect" size="middle">
              <template #icon><ReloadOutlined :spin="store.refreshing" /></template>
              刷新列表
            </a-button>
            <a-button 
              type="primary" 
              @click="clearCompletedTasks" 
              class="btn-with-effect" 
              size="middle" 
              style="margin-left: 12px" 
              danger
              :disabled="!hasCompletedTasks"
            >
              <template #icon><DeleteOutlined /></template>
              清除已完成
            </a-button>
            <a-button type="primary" @click="showNewTaskModal" style="margin-left: 12px" class="btn-with-effect" size="middle">
              <template #icon><PlusOutlined /></template>
              新建任务
            </a-button>
          </div>
        </div>
        
        <a-divider style="margin: 12px 0" />
        
        <!-- 任务列表 -->
        <div class="task-list">
          <a-row :gutter="[20, 20]" class="card-row">
            <a-col :xs="24" :sm="12" :md="8" :lg="8" :xl="6" :xxl="6" v-for="task in store.sortedTasks" :key="task.id">
              <a-card 
                class="task-card fade-in" 
                :hoverable="true"
                :bordered="true">
                <div class="task-header">
                  <div class="task-title">
                    <div class="url-container">
                      <a-typography-link :href="task.url" target="_blank" class="task-url" :title="task.url">
                        {{ truncateText(task.url, 80) }}
                      </a-typography-link>
                    </div>
                    <a-tag :color="statusColors[task.status]" class="status-tag">
                      <span class="status-icon">
                        <LoadingOutlined v-if="task.status === 'downloading' || task.status === 'converting'" spin />
                        <CheckCircleOutlined v-else-if="task.status === 'success'" />
                        <CloseCircleOutlined v-else-if="task.status === 'failed'" />
                        <HourglassOutlined v-else />
                      </span>
                      {{ statusTexts[task.status] || task.status }}
                    </a-tag>
                  </div>
                  <div class="task-actions">
                    <a-tooltip title="重试下载">
                      <a-button 
                        v-if="task.status === 'failed'"
                        type="primary" 
                        shape="circle" 
                        size="small"
                        @click="retryTask(task.id)"
                        class="action-button retry-button"
                      >
                        <template #icon><ReloadOutlined /></template>
                      </a-button>
                    </a-tooltip>
                    <a-tooltip title="删除任务">
                      <a-button 
                        type="primary" 
                        danger 
                        shape="circle" 
                        size="small"
                        @click="confirmDelete(task.id)"
                        class="action-button delete-button"
                      >
                        <template #icon><DeleteOutlined /></template>
                      </a-button>
                    </a-tooltip>
                  </div>
                </div>
                
                <div class="task-info">
                  <div class="info-item location">
                    <span class="info-icon"><FolderOutlined /></span> 
                    <span class="info-text">{{ task.output }}</span>
                  </div>
                  <div class="info-item threads">
                    <span class="info-icon"><TeamOutlined /></span> 
                    <span class="info-text">线程: {{ task.c }}</span>
                  </div>
                  <div v-if="task.fileName" class="info-item file-name">
                    <span class="info-icon"><FileOutlined /></span> 
                    <span class="info-text">{{ task.fileName }}</span>
                  </div>
                </div>
                
                <div class="task-progress">
                  <div class="progress-header">
                    <div class="progress-percent">{{ task.progress }}%</div>
                    <div class="task-message" :style="{ color: getMessageColor(task.status) }">
                      {{ task.message }}
                    </div>
                  </div>
                  
                  <!-- 添加下载速度显示 -->
                  <div v-if="task.status === 'downloading'" class="download-speed">
                    <span class="speed-icon"><ThunderboltOutlined /></span>
                    <span class="speed-text">{{ formatSpeed(task.speed) }}</span>
                    <a-tag color="#1890ff" class="speed-tag">速度</a-tag>
                  </div>
                  
                  <!-- 文件大小显示，无论任务状态如何都显示 -->
                  <div v-if="task.totalSize" class="download-size" :class="{
                    'size-success': task.status === 'success',
                    'size-downloading': task.status === 'downloading',
                    'size-failed': task.status === 'failed'
                  }">
                    <span class="size-icon"><DatabaseOutlined /></span>
                    <span class="size-text">{{ formatFileSize(task.totalSize) }}</span>
                    <a-tag :color="getSizeTagColor(task.status)" class="size-tag">大小</a-tag>
                  </div>
                  
                  <a-progress 
                    :percent="task.progress" 
                    :status="getProgressStatus(task.status)"
                    :stroke-color="getProgressColor(task.status)"
                    :format="() => ''"
                    size="default"
                    :stroke-width="8"
                  />
                </div>
              </a-card>
            </a-col>
          </a-row>
          
          <div v-if="store.tasks.length === 0" class="empty-state fade-in">
            <div class="empty-icon">
              <InboxOutlined />
            </div>
            <div class="empty-text">暂无下载任务</div>
            <a-button type="primary" @click="showNewTaskModal" class="btn-with-effect">
              <PlusOutlined /> 创建第一个下载任务
            </a-button>
          </div>
        </div>
      </a-card>
    </a-spin>
    
    <!-- 全局操作提示 -->
    <a-back-top :visibilityHeight="100" :style="{ right: '24px' }">
      <div class="back-top-button">
        <a-button type="primary" shape="circle" size="large">
          <template #icon><UpOutlined /></template>
        </a-button>
      </div>
    </a-back-top>
    
    <!-- 新建下载弹窗 -->
    <a-modal
      v-model:open="modalVisible"
      title="新建下载任务"
      :confirm-loading="store.loading"
      @ok="submitForm"
      :ok-text="'开始下载'"
      :cancel-text="'取消'"
      centered
      :styles="{ body: { paddingBottom: 16 } }"
      :width="700"
      class="download-modal"
    >
      <div class="fade-in-fast">
        <a-form 
          ref="formRef"
          :model="formState"
          layout="vertical"
          @finish="onFinish"
        >
          <!-- 基本配置 -->
          <div class="form-section">
            <a-typography-title :level="5" style="margin: 0 0 16px 0">
              <span class="section-icon"><LinkOutlined /></span> 基本配置
            </a-typography-title>
            
            <a-form-item 
              name="url" 
              label="M3U8 链接" 
              :rules="[{ required: true, whitespace: true, message: '请输入m3u8链接' }]"
            >
              <a-input 
                v-model:value="formState.url" 
                placeholder="请输入 m3u8 链接，如 https://example.com/video.m3u8" 
                size="large"
                :prefix="h(LinkOutlined)"
                class="input-with-effect"
                ref="urlInputRef"
              >
                <template #addonAfter>
                  <span
                    class="paste-button"
                    title="粘贴剪贴板"
                    @click="pasteUrl"
                    id="url-paste-btn"
                  >
                    <svg width="14" height="14" viewBox="0 0 1024 1024">
                      <path d="M832 112H724V72c0-22.1-17.9-40-40-40H340c-22.1 0-40 17.9-40 40v40H192c-35.3 0-64 28.7-64 64v712c0 35.3 28.7 64 64 64h640c35.3 0 64-28.7 64-64V176c0-35.3-28.7-64-64-64zM372 80h280v32H372V80zm484 808c0 17.7-14.3 32-32 32H200c-17.7 0-32-14.3-32-32V184c0-17.7 14.3-32 32-32h88v40c0 22.1 17.9 40 40 40h304c22.1 0 40-17.9 40-40v-40h88c17.7 0 32 14.3 32 32v704z" fill="#1890ff"/>
                    </svg>
                    <span>粘贴</span>
                  </span>
                </template>
              </a-input>
              <template #extra>
                <span class="form-extra">支持 http/https 开头的 m3u8 链接</span>
              </template>
            </a-form-item>
            
            <a-form-item 
              name="output" 
              label="下载位置" 
              :rules="[{ required: true, whitespace: true, message: '请输入下载保存目录' }]"
            >
              <a-input 
                v-model:value="formState.output" 
                placeholder="如 /data/videos" 
                size="large"
                :prefix="h(FolderOutlined)"
                class="input-with-effect"
                ref="outputInputRef"
              >
                <template #addonAfter>
                  <span
                    class="browse-button"
                    title="选择文件夹"
                    @click="showFolderTree"
                    id="output-browse-btn"
                  >
                    <FolderOpenOutlined style="font-size: 14px;" />
                    <span>选择</span>
                  </span>
                </template>
              </a-input>
              <template #extra>
                <span class="form-extra">点击选择按钮浏览并选择下载目录</span>
              </template>
            </a-form-item>
            
            <a-form-item 
              name="customFileName" 
              label="自定义文件名"
            >
              <a-input 
                v-model:value="formState.customFileName" 
                placeholder="输入自定义文件名，例如：my_video.mp4" 
                size="large"
                :prefix="h(FileOutlined)"
                class="input-with-effect"
                ref="customFileNameInputRef"
              >
                <template #addonAfter>
                  <span
                    class="paste-button"
                    title="粘贴剪贴板"
                    @click="pasteCustomFileName"
                    id="filename-paste-btn"
                  >
                    <svg width="14" height="14" viewBox="0 0 1024 1024">
                      <path d="M832 112H724V72c0-22.1-17.9-40-40-40H340c-22.1 0-40 17.9-40 40v40H192c-35.3 0-64 28.7-64 64v712c0 35.3 28.7 64 64 64h640c35.3 0 64-28.7 64-64V176c0-35.3-28.7-64-64-64zM372 80h280v32H372V80zm484 808c0 17.7-14.3 32-32 32H200c-17.7 0-32-14.3-32-32V184c0-17.7 14.3-32 32-32h88v40c0 22.1 17.9 40 40 40h304c22.1 0 40-17.9 40-40v-40h88c17.7 0 32 14.3 32 32v704z" fill="#1890ff"/>
                    </svg>
                    <span>粘贴</span>
                  </span>
                </template>
              </a-input>
              <template #extra>
                <span class="form-extra">可选，不填则使用默认文件名，文件名会自动处理重复</span>
              </template>
            </a-form-item>
          </div>
          
          <!-- 高级配置 -->
          <div class="form-section">
            <a-typography-title :level="5" style="margin: 16px 0">
              <span class="section-icon"><SettingOutlined /></span> 高级配置
            </a-typography-title>
            
            <div class="option-row">
              <a-form-item 
                name="c" 
                label="下载线程" 
                :rules="[{ required: true, type: 'number', min: 1, max: 128, message: '线程数需为1-128' }]"
                style="flex: 1; margin-right: 16px"
              >
                <a-input-number
                  v-model:value="formState.c"
                  :min="1"
                  :max="128"
                  style="width: 100%"
                  size="large"
                  placeholder="25"
                  class="input-with-effect"
                />
                <template #extra>
                  <span class="form-extra">建议10-50，过高可能影响稳定性</span>
                </template>
              </a-form-item>
              
              <a-form-item style="flex: 1; margin-top: 30px">
                <div class="checkbox-group">
                  <a-checkbox v-model:checked="formState.convertToMp4" class="option-checkbox">
                    <span class="checkbox-label">
                      <VideoCameraOutlined style="marginRight: 8px" />
                      下载为MP4格式
                    </span>
                  </a-checkbox>
                  <a-checkbox v-model:checked="formState.deleteTs" class="option-checkbox">
                    <span class="checkbox-label">
                      <DeleteOutlined style="marginRight: 8px" />
                      合并后删除分片
                    </span>
                  </a-checkbox>
                </div>
              </a-form-item>
            </div>
            
            <a-alert 
              type="info" 
              show-icon 
              class="download-info"
              message="优化说明" 
              description="启用了自动分块处理，能更好地处理大文件下载；文件名冲突时会自动添加序号，不会覆盖旧文件。"
            />
          </div>
        </a-form>
      </div>
    </a-modal>
    
    <!-- 文件夹选择弹窗 -->
    <a-modal
      v-model:open="folderTreeVisible"
      title="选择下载文件夹"
      :confirm-loading="folderTreeLoading"
      @ok="selectFolder"
      :ok-text="'确定选择'"
      :cancel-text="'取消'"
      centered
      :width="600"
      class="folder-tree-modal"
    >
      <div class="folder-tree-container">
        <div class="current-path">
          <span class="path-label">当前位置：</span>
          <a-tag color="blue" class="selected-path">
            <FolderOutlined />
            {{ selectedPath || '未选择' }}
          </a-tag>
          <a-button 
            v-if="folderBreadcrumbs.length > 1" 
            type="link" 
            size="small" 
            @click="goToParentFolder" 
            class="parent-folder-btn"
          >
            <UpOutlined /> 返回上级
          </a-button>
        </div>
        
        <a-spin :spinning="folderTreeLoading" tip="加载文件夹...">
          <div class="folder-list-container">
            <a-empty 
              v-if="folderList.length === 0 && !folderTreeLoading" 
              description="暂无可选择的文件夹" 
              class="empty-folders"
            >
              <template #image>
                <FolderOutlined class="empty-icon" />
              </template>
              <div class="root-path-info">
                根路径: {{ rootPath || '未设置' }}
              </div>
            </a-empty>
            
            <div v-else class="folder-grid">
              <div 
                v-for="folder in folderList" 
                :key="folder.path" 
                class="folder-item"
                :class="{ 'folder-selected': selectedPath === folder.path }"
                @click="selectFolderItem(folder.path)"
                @dblclick="navigateToFolder(folder.path)"
              >
                <div class="folder-icon-wrapper">
                  <FolderOutlined class="folder-icon" />
                </div>
                <div class="folder-name" :title="folder.name">{{ folder.name }}</div>
              </div>
            </div>
          </div>
        </a-spin>
        
        <div class="breadcrumb-container" v-if="folderBreadcrumbs.length > 0">
          <a-breadcrumb>
            <a-breadcrumb-item v-for="(crumb, index) in folderBreadcrumbs" :key="index">
              <a @click="navigateToBreadcrumb(crumb.path)" :class="{'active-crumb': crumb.path === selectedPath}">
                <span v-if="index === 0"><HomeOutlined /></span>
                <span v-else>{{ crumb.name }}</span>
              </a>
            </a-breadcrumb-item>
          </a-breadcrumb>
        </div>
        
        <div class="tree-actions">
          <a-button @click="refreshFolderList" :loading="folderTreeLoading" size="small">
            <template #icon><ReloadOutlined /></template>
            刷新
          </a-button>
          <a-button @click="createNewFolder" type="primary" size="small">
            <template #icon><PlusOutlined /></template>
            新建文件夹
          </a-button>
        </div>
      </div>
    </a-modal>
    
    <!-- 新建文件夹弹窗 -->
    <a-modal
      v-model:open="createFolderVisible"
      title="新建文件夹"
      @ok="confirmCreateFolder"
      :ok-text="'创建'"
      :cancel-text="'取消'"
      centered
      :width="400"
    >
      <a-form layout="vertical">
        <a-form-item label="文件夹名称">
          <a-input 
            v-model:value="newFolderName"
            placeholder="请输入文件夹名称"
            @keyup.enter="confirmCreateFolder"
            ref="newFolderInputRef"
          />
        </a-form-item>
        <a-form-item label="创建位置">
          <a-input 
            :value="selectedPath || rootPath"
            readonly
            :prefix="h(FolderOutlined)"
          />
        </a-form-item>
      </a-form>
    </a-modal>
    
    <!-- 删除确认弹窗 -->
    <a-modal
      v-model:open="deleteModalVisible"
      title="确认删除任务"
      @ok="handleDelete"
      :okButtonProps="{ danger: true, style: { backgroundColor: '#ff4d4f', borderColor: '#ff4d4f' } }"
      okText="确定删除"
      cancelText="取消"
      centered
      :width="420"
    >
      <div class="delete-confirm fade-in-fast">
        <ExclamationCircleOutlined class="warning-icon" />
        <div class="delete-message">
          <p class="delete-title">确定要删除该下载任务吗？</p>
          <p class="delete-desc">此操作将永久删除该任务及其相关文件，且无法恢复。</p>
        </div>
      </div>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, onBeforeUnmount, h, computed, nextTick } from 'vue'
import { useTaskStore } from '../stores/taskStore'
import { message, Modal } from 'ant-design-vue'
import { 
  PlusOutlined, 
  DeleteOutlined, 
  ReloadOutlined, 
  ExclamationCircleOutlined,
  DownloadOutlined,
  FileOutlined,
  FolderOutlined,
  FolderOpenOutlined,
  LinkOutlined,
  TeamOutlined,
  InboxOutlined,
  VideoCameraOutlined,
  LoadingOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  SettingOutlined,
  UpOutlined,
  ThunderboltOutlined,
  HomeOutlined,
  DatabaseOutlined,
  HourglassOutlined
} from '@ant-design/icons-vue'
import axios from 'axios'

// 状态管理
const store = useTaskStore()
const formRef = ref(null)
const modalVisible = ref(false)
const deleteModalVisible = ref(false)
const currentTaskId = ref(null)
let refreshInterval = null
const urlInputRef = ref(null)
const outputInputRef = ref(null)

// 文件夹选择相关状态
const folderTreeVisible = ref(false)
const folderTreeLoading = ref(false)
const folderTreeData = ref([])
const folderList = ref([]) // 当前目录下的文件夹列表
const selectedPath = ref('')
const rootPath = ref('')
const initialPath = ref('') // 记住刚打开窗口时的路径
const folderBreadcrumbs = ref([]) // 文件夹面包屑导航

// 新建文件夹相关状态
const createFolderVisible = ref(false)
const newFolderName = ref('')
const newFolderInputRef = ref(null)

// 全局限速值
const speedLimit = ref(0)

// 格式化限速
const formatLimit = (limit) => {
  if (limit >= 1024) {
    return `${(limit / 1024).toFixed(limit % 1024 === 0 ? 0 : 1)}MB/s`
  }
  return `${limit}KB/s`
}

// 表单状态
const formState = reactive({
  url: '',
  output: '',
  c: 25,
  customFileName: '',
  deleteTs: true,
  convertToMp4: true
})

// 状态标签颜色映射
const statusColors = {
  'pending': 'blue',
  'downloading': 'processing',
  'converting': 'purple',
  'success': 'success',
  'failed': 'error',
  'unfinished': 'orange'
}

// 状态文本映射
const statusTexts = {
  'pending': '等待下载',
  'downloading': '下载中',
  'converting': '格式转换中',
  'success': '下载完成',
  'failed': '下载失败',
  'unfinished': '下载未完成'
}

// localStorage键名
const LAST_DOWNLOAD_FORM_KEY = 'last_download_form_data'

// 初始化
onMounted(() => {
  store.initialLoading = true
  store.fetchTasks()
    .finally(() => {
      store.initialLoading = false
    })
  
  // 加载全局设置，获取限速
  fetchGlobalSettings()
  
  // 每1秒刷新一次任务列表
  refreshInterval = setInterval(() => {
    store.fetchTasks()
    
    // 每15秒更新一次全局设置（包括限速信息）
    if (new Date().getSeconds() % 15 === 0) {
      fetchGlobalSettings()
    }
  }, 1000)
})

// 组件卸载时清除定时器
onBeforeUnmount(() => {
  clearInterval(refreshInterval)
})

// 获取文件树数据，修改为平铺展示
const fetchFolderTree = async (path = '') => {
  try {
    folderTreeLoading.value = true
    
    const response = await axios.get('/api/folders', {
      params: { path }
    })
    
    if (response.data.success) {
      const data = response.data.data
      
      // 只在第一次加载时设置rootPath，避免后续导航时覆盖根路径
      if (!rootPath.value) {
        rootPath.value = data.rootPath || ''
      }
      
      // 提取当前目录下的文件夹
      const extractFolders = (folders) => {
        if (!folders || !Array.isArray(folders)) {
          return []
        }
        return folders.map(folder => ({
          name: folder.name,
          path: folder.path,
          hasChildren: folder.children && folder.children.length > 0
        }))
      }
      
      // 更新当前目录下的文件夹列表
      folderList.value = extractFolders(data.folders || [])
      
      // 如果selectedPath还没有设置，或者表单的output为空，则使用返回的rootPath
      if (!selectedPath.value || !formState.output) {
        selectedPath.value = rootPath.value
      }
      
      // 更新面包屑导航
      updateBreadcrumbs(selectedPath.value)
    } else {
      message.error(response.data.message || '获取文件夹列表失败')
      folderList.value = []
    }
  } catch (error) {
    // 处理API返回的错误信息
    if (error.response && error.response.data && error.response.data.message) {
      message.error(error.response.data.message);
    } else {
      message.error('获取文件夹列表失败');
    }
    folderList.value = []
  } finally {
    folderTreeLoading.value = false
  }
}

// 选择文件夹项
const selectFolderItem = (path) => {
  selectedPath.value = path
}

// 双击导航到文件夹
const navigateToFolder = async (path) => {
  selectedPath.value = path
  await fetchFolderTree(path)
}

// 点击面包屑导航
const navigateToBreadcrumb = async (path) => {
  selectedPath.value = path
  await fetchFolderTree(path)
}

// 更新面包屑导航
const updateBreadcrumbs = (path) => {
  if (!path) {
    folderBreadcrumbs.value = []
    return
  }
  
  // 分割路径
  const parts = path.split('/')
  let currentPath = ''
  
  // 创建面包屑数组
  folderBreadcrumbs.value = parts.map((part, index) => {
    if (index === 0 && part === '') {
      // 根目录
      currentPath = '/'
      return { name: '根目录', path: '/' }
    }
    
    if (part === '') return null
    
    currentPath = currentPath === '/' 
      ? `/${part}` 
      : `${currentPath}/${part}`
      
    return { name: part, path: currentPath }
  }).filter(Boolean) // 过滤掉null值
  
  // 如果是Windows路径（如C:\），特殊处理
  if (/^[A-Z]:\\/.test(path)) {
    const drive = path.substring(0, 2)
    folderBreadcrumbs.value = [
      { name: drive, path: drive + '\\' }
    ]
    
    const winParts = path.substring(3).split('\\')
    let winPath = drive + '\\'
    
    winParts.forEach(part => {
      if (part) {
        winPath += part + '\\'
        folderBreadcrumbs.value.push({
          name: part,
          path: winPath
        })
      }
    })
  }
}

// 返回上级文件夹
const goToParentFolder = async () => {
  if (!selectedPath.value) return
  
  let parentPath = ''
  
  // 处理Windows路径和Unix路径
  if (selectedPath.value.includes('\\')) {
    // Windows路径
    const parts = selectedPath.value.split('\\')
    parts.pop() // 移除最后一个部分
    parentPath = parts.join('\\')
    
    // 如果只剩下盘符，确保添加反斜杠
    if (/^[A-Z]:$/.test(parentPath)) {
      parentPath += '\\'
    }
  } else {
    // Unix路径
    const parts = selectedPath.value.split('/')
    parts.pop() // 移除最后一个部分
    parentPath = parts.join('/')
    
    // 确保根路径是 /
    if (parentPath === '') {
      parentPath = '/'
    }
  }
  
  // 更新选中路径
  selectedPath.value = parentPath
  
  // 加载父文件夹的内容
  await fetchFolderTree(parentPath)
}

// 刷新文件夹列表
const refreshFolderList = () => {
  fetchFolderTree(selectedPath.value)
}

// 显示文件夹选择弹窗
const showFolderTree = async () => {
  // 添加按钮动画效果
  const button = document.getElementById('output-browse-btn');
  button?.classList.add('browse-animation');
  setTimeout(() => {
    button?.classList.remove('browse-animation');
  }, 500);
  
  // 记住初始路径（刚打开窗口时的路径）
  initialPath.value = formState.output || ''
  
  // 设置当前选择的路径为表单中的output值
  if (formState.output) {
    selectedPath.value = formState.output
  }
  
  // 重置根路径，确保每次打开时都能正确获取
  rootPath.value = ''
  
  folderTreeVisible.value = true
  
  // 加载文件夹列表
  await fetchFolderTree(initialPath.value)
}

// 确认选择文件夹
const selectFolder = () => {
  if (selectedPath.value) {
    formState.output = selectedPath.value
    folderTreeVisible.value = false
    message.success('已选择下载位置: ' + selectedPath.value)
  } else {
    message.warning('请先选择一个文件夹')
  }
}

// 确认创建文件夹
const confirmCreateFolder = async () => {
  if (!newFolderName.value.trim()) {
    message.warning('请输入文件夹名称')
    return
  }
  
  try {
    const createPath = selectedPath.value || rootPath.value
    
    const response = await axios.post('/api/folders/create', {
      path: createPath,
      name: newFolderName.value.trim()
    })
    
    if (response.data.success) {
      message.success('文件夹创建成功')
      createFolderVisible.value = false
      
      // 刷新当前目录的文件夹列表
      await fetchFolderTree(selectedPath.value)
    } else {
      // 使用后端返回的具体错误消息
      message.error(response.data.message || '创建文件夹失败')
    }
  } catch (error) {
    // 处理网络错误或其他异常
    if (error.response && error.response.data && error.response.data.message) {
      // 如果是HTTP错误响应，使用响应中的错误消息
      message.error(error.response.data.message)
    } else {
      // 其他类型的错误（网络错误等）
      message.error('网络错误，请检查连接后重试')
    }
  }
}

// 显示新建文件夹弹窗
const createNewFolder = () => {
  newFolderName.value = ''
  createFolderVisible.value = true
  nextTick(() => {
    newFolderInputRef.value?.focus()
  })
}

// 获取进度条状态
const getProgressStatus = (status) => {
  switch (status) {
    case 'downloading': return 'active'
    case 'converting': return 'active'
    case 'success': return 'success'
    case 'failed': return 'exception'
    default: return 'normal'
  }
}

// 获取进度条颜色
const getProgressColor = (status) => {
  switch (status) {
    case 'downloading': return { from: '#108ee9', to: '#1890ff' }
    case 'converting': return { from: '#722ed1', to: '#9254de' }
    case 'success': return { from: '#52c41a', to: '#73d13d' }
    case 'failed': return { from: '#f5222d', to: '#ff4d4f' }
    default: return { from: '#faad14', to: '#ffc53d' }
  }
}

// 获取消息文本颜色
const getMessageColor = (status) => {
  switch (status) {
    case 'success': return '#52c41a'
    case 'failed': return '#f5222d'
    case 'converting': return '#722ed1'
    case 'downloading': return '#1890ff'
    default: return '#faad14'
  }
}

// 截断长文本
const truncateText = (text, maxLength) => {
  if (!text) return '';
  return text.length > maxLength ? text.substring(0, maxLength) + '...' : text;
}

// 手动刷新任务列表
const refreshTasks = () => {
  store.fetchTasks()
}

// 从localStorage获取上次表单数据（只获取链接和文件名）
const getLastFormData = () => {
  try {
    const savedData = localStorage.getItem(LAST_DOWNLOAD_FORM_KEY)
    if (savedData) {
      const parsedData = JSON.parse(savedData);
      // 只提取链接和文件名
      return {
        url: parsedData.url || '',
        customFileName: parsedData.customFileName || ''
      };
    }
  } catch (err) {
    // 静默处理错误
  }
  return null // 返回null表示没有本地存储数据
}

// 获取默认设置
const getDefaultSettings = async () => {
  try {
    const response = await axios.get('/api/settings')
    if (response.data.success) {
      const data = response.data.data || {}
      return {
        output: data.defaultOutputPath || '',
        c: data.defaultThreadCount || 25,
        deleteTs: data.defaultDeleteTs !== undefined ? data.defaultDeleteTs : true,
        convertToMp4: data.defaultConvertToMp4 !== undefined ? data.defaultConvertToMp4 : true
      }
    }
  } catch (error) {
    // 静默处理错误
  }
  // 如果获取失败，返回硬编码的默认值
  return { c: 25, deleteTs: true, convertToMp4: true }
}

// 显示新建下载弹窗
const showNewTaskModal = async () => {
  // 获取上次保存的表单数据（仅用于m3u8链接和自定义文件名）
  const lastData = getLastFormData()
  
  // 获取服务器默认设置
  const defaultSettings = await getDefaultSettings()
  
  // 设置表单字段
  // 对于m3u8链接和自定义文件名，使用上次保存的值（如果有）
  formState.url = lastData?.url || '';
  formState.customFileName = lastData?.customFileName || '';
  
  // 对于其他配置项，始终使用服务器的全局配置
  formState.output = defaultSettings.output || '';
  formState.c = defaultSettings.c || 25;
  formState.deleteTs = defaultSettings.deleteTs !== undefined ? Boolean(defaultSettings.deleteTs) : true;
  formState.convertToMp4 = defaultSettings.convertToMp4 !== undefined ? Boolean(defaultSettings.convertToMp4) : true;
  
  modalVisible.value = true
}

// 提交表单
const submitForm = () => {
  // 直接获取当前表单状态，确保布尔值正确传递
  const currentFormData = {
    url: formState.url,
    output: formState.output,
    c: formState.c,
    customFileName: formState.customFileName,
    deleteTs: Boolean(formState.deleteTs),
    convertToMp4: Boolean(formState.convertToMp4)
  };
  
  formRef.value.validate()
    .then(() => {
      onFinish(currentFormData);
    })
    .catch(error => {
      console.error('验证失败:', error);
      message.error('表单验证失败，请检查输入内容');
    });
}

// 表单完成回调
const onFinish = async (values) => {
  const result = await store.createTask(values)
  
  if (result.success) {
    saveFormData(values) // 保存表单数据
    message.success('下载任务已创建')
    modalVisible.value = false
  } else {
    message.error(result.message || '创建下载任务失败')
  }
}

// 显示删除确认对话框
const confirmDelete = (id) => {
  currentTaskId.value = id
  deleteModalVisible.value = true
}

// 执行删除操作
const handleDelete = async () => {
  if (currentTaskId.value) {
    message.loading({ content: '正在停止下载并删除文件...', key: 'deleteTask', duration: 0 })
    
    const result = await store.deleteTask(currentTaskId.value)
    
    if (result.success) {
      message.success({ content: '删除任务成功', key: 'deleteTask', duration: 2 })
    } else {
      message.error({ content: result.message || '删除任务失败', key: 'deleteTask', duration: 3 })
    }
    
    deleteModalVisible.value = false
    currentTaskId.value = null
  }
}

// 重试任务
const retryTask = async (id) => {
  message.loading({ content: '正在重新开始下载...', key: 'retryTask', duration: 0 })
  const result = await store.retryTask(id)
  
  if (result.success) {
    message.success({ content: '已重新开始下载', key: 'retryTask', duration: 2 })
  } else {
    message.error({ content: result.message || '重试下载失败', key: 'retryTask', duration: 3 })
  }
}

// 格式化下载速度
const formatSpeed = (speed) => {
  if (!speed) return '0 KB/s';
  
  // 速度是字节/秒，转换为更友好的单位
  if (speed < 1024) {
    return `${speed.toFixed(1)} B/s`;
  } else if (speed < 1024 * 1024) {
    return `${(speed / 1024).toFixed(1)} KB/s`;
  } else if (speed < 1024 * 1024 * 1024) {
    return `${(speed / (1024 * 1024)).toFixed(1)} MB/s`;
  } else {
    return `${(speed / (1024 * 1024 * 1024)).toFixed(1)} GB/s`;
  }
}

const pasteUrl = async () => {
  try {
    // 添加按钮动画效果
    const button = document.getElementById('url-paste-btn');
    button?.classList.add('paste-animation');
    setTimeout(() => {
      button?.classList.remove('paste-animation');
    }, 500);
    
    const text = await navigator.clipboard.readText()
    formState.url = text
    message.success('已粘贴剪贴板内容')
  } catch (e) {
    message.error('无法读取剪贴板内容，请检查浏览器权限')
  }
}

const pasteCustomFileName = async () => {
  try {
    // 添加按钮动画效果
    const button = document.getElementById('filename-paste-btn');
    button?.classList.add('paste-animation');
    setTimeout(() => {
      button?.classList.remove('paste-animation');
    }, 500);
    
    const text = await navigator.clipboard.readText()
    formState.customFileName = text
    message.success('已粘贴剪贴板内容')
  } catch (e) {
    message.error('无法读取剪贴板内容，请检查浏览器权限')
  }
}

// 保存表单数据到localStorage（只保存链接和文件名）
const saveFormData = (values) => {
  try {
    // 只保存m3u8链接和自定义文件名
    const dataToSave = {
      url: values.url,
      customFileName: values.customFileName
    };
    localStorage.setItem(LAST_DOWNLOAD_FORM_KEY, JSON.stringify(dataToSave))
  } catch (err) {
    console.error('保存表单数据失败:', err)
  }
}

// 清除已完成任务
const clearCompletedTasks = async () => {
  // 使用确认对话框
  Modal.confirm({
    title: '确认清除已完成任务',
    icon: h(ExclamationCircleOutlined, { style: { color: '#ff4d4f' } }),
    content: '确定要清除所有已完成的下载任务记录吗？',
    okText: '确认清除',
    cancelText: '取消',
    okButtonProps: { danger: true },
    onOk: async () => {
      message.loading({ content: '正在清除已完成任务...', key: 'clearTasks', duration: 0 })
      
      const result = await store.clearCompletedTasks()
      
      if (result.success) {
        message.success({ content: result.message || '已清除完成任务', key: 'clearTasks', duration: 2 })
      } else {
        message.error({ content: result.message || '清除任务失败', key: 'clearTasks', duration: 3 })
      }
    }
  })
}

// 判断是否有已完成任务的计算属性
const hasCompletedTasks = computed(() => {
  return store.tasks.some(task => task.status === 'success')
})

// 获取全局设置的函数
const fetchGlobalSettings = () => {
  axios.get('/api/settings').then(res => {
    if (res.data.success) {
      speedLimit.value = res.data.data?.downloadSpeedLimit || 0
    }
  }).catch(() => {})
}

// 格式化文件大小
const formatFileSize = (size) => {
  if (!size || size === 0) return '未知';
  
  // 假设size是以字节为单位
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let sizeValue = size;
  let unitIndex = 0;
  
  // 逐级转换到合适的单位
  while (sizeValue >= 1024 && unitIndex < units.length - 1) {
    sizeValue /= 1024;
    unitIndex++;
  }
  
  // 根据大小决定小数位数
  let decimals = 2;
  if (unitIndex === 0) decimals = 0; // 字节不需要小数
  if (unitIndex === 1) decimals = 1; // KB保留1位小数
  
  return `${sizeValue.toFixed(decimals)} ${units[unitIndex]}`;
}

// 获取文件大小标签颜色
const getSizeTagColor = (status) => {
  switch (status) {
    case 'success': return '#52c41a'
    case 'failed': return '#f5222d'
    case 'downloading': return '#722ed1'
    default: return '#faad14'
  }
}
</script>

<style scoped>
/* 自定义淡入动画 */
@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.fade-in {
  animation: fadeIn 0.5s ease-out;
}

.fade-in-fast {
  animation: fadeIn 0.3s ease-out;
}

/* 粘贴动画效果 */
@keyframes paste-pulse {
  0% {
    box-shadow: 0 0 0 0 rgba(24, 144, 255, 0.7);
  }
  70% {
    box-shadow: 0 0 0 10px rgba(24, 144, 255, 0);
  }
  100% {
    box-shadow: 0 0 0 0 rgba(24, 144, 255, 0);
  }
}

.paste-animation {
  animation: paste-pulse 0.5s 1;
  background-color: #1890ff !important;
  color: white !important;
}

.paste-animation svg path {
  fill: white !important;
}

/* 浏览按钮动画效果 */
@keyframes browse-pulse {
  0% {
    box-shadow: 0 0 0 0 rgba(114, 46, 209, 0.7);
  }
  70% {
    box-shadow: 0 0 0 10px rgba(114, 46, 209, 0);
  }
  100% {
    box-shadow: 0 0 0 0 rgba(114, 46, 209, 0);
  }
}

.browse-animation {
  animation: browse-pulse 0.5s 1;
  background-color: #722ed1 !important;
  color: white !important;
}

.download-manager {
  width: 100%;
  margin: 0 auto;
  padding: 0;
  max-width: 100%;
}

.main-card {
  border-radius: 10px;
  box-shadow: 0 2px 15px rgba(0, 0, 0, 0.08);
  transition: all 0.3s;
  padding: 0;
  margin: 0;
}

.main-card :deep(.ant-card-body) {
  padding: 20px !important;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 0 8px;
}

.title-section {
  flex: 1;
}

.title-icon {
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0% {
    opacity: 1;
  }
  50% {
    opacity: 0.6;
  }
  100% {
    opacity: 1;
  }
}

.action-buttons {
  display: flex;
  align-items: center;
  margin-right: 0;
}

.task-list {
  margin-top: 16px;
}

.task-list-enter-active,
.task-list-leave-active {
  transition: all 0.5s;
}

.task-list-enter-from,
.task-list-leave-to {
  opacity: 0;
  transform: translateY(30px);
}

.card-row {
  margin-bottom: 0;
  padding: 0 8px;
}

/* 添加一个在大屏上的最大宽度限制 */
@media (min-width: 1920px) {
  .card-row {
    max-width: 2400px;
    margin: 0 auto;
  }
}

.task-card {
  border-radius: 10px;
  transition: all 0.3s ease;
  border: 1px solid #f0f0f0;
  overflow: hidden;
  height: 100%;
  display: flex;
  flex-direction: column;
  animation: fadeIn 0.5s ease-in-out;
}

.task-card:hover {
  transform: translateY(-3px);
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.12);
  border-color: #e6f7ff;
}

.task-card :deep(.ant-card-body) {
  padding: 16px !important;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.task-title {
  display: flex;
  align-items: flex-start;
  flex-direction: column;
  flex: 1;
  overflow: hidden;
}

.url-container {
  max-width: 100%;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  flex-grow: 1;
  margin-bottom: 6px;
  width: 100%;
}

.task-url {
  font-weight: 500;
  font-size: 15px;
  color: #1890ff;
  transition: color 0.3s;
  display: inline-block;
  max-width: 100%;
}

.task-url:hover {
  color: #40a9ff;
  text-decoration: underline;
}

.task-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 12px;
  width: 100%;
}

.task-actions {
  display: flex;
  gap: 8px;
}

.action-button {
  transition: all 0.3s ease;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.1);
}

.action-button:hover {
  transform: scale(1.15);
}

.folder-button {
  background-color: #1890ff;
}

.delete-button {
  background-color: #ff4d4f;
}

.task-info {
  display: flex;
  flex-wrap: wrap;
  margin-bottom: 12px;
  font-size: 13px;
  color: #666;
  gap: 8px;
}

.info-item {
  margin-right: 16px;
  margin-bottom: 6px;
  display: flex;
  align-items: center;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
}

.info-icon {
  margin-right: 6px;
  color: #1890ff;
  flex-shrink: 0;
}

.file-name .info-icon {
  color: #52c41a;
}

.info-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.location {
  width: 100%;
  margin-bottom: 4px;
}

.file-name {
  font-weight: 500;
  color: #1890ff;
  width: 100%;
  margin-top: 2px;
}

.task-message {
  font-size: 14px;
  max-width: 75%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.task-progress {
  margin-top: auto;
  background-color: #f9f9f9;
  padding: 12px;
  border-radius: 8px;
  transition: background-color 0.3s;
}

.task-card:hover .task-progress {
  background-color: #f0f7ff;
}

.progress-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.progress-percent {
  font-weight: bold;
  font-size: 15px;
  color: #1890ff;
}

a-progress :deep(.ant-progress-outer) {
  padding-right: 0 !important;
  margin-right: 0 !important;
}

.task-card :deep(.ant-progress) {
  line-height: 1;
}

.empty-state {
  padding: 60px 0;
  text-align: center;
  background-color: #fafafa;
  border-radius: 10px;
  margin-top: 20px;
}

.empty-icon {
  font-size: 64px;
  color: #bfbfbf;
  margin-bottom: 16px;
  animation: float 3s ease-in-out infinite;
}

@keyframes float {
  0% {
    transform: translateY(0px);
  }
  50% {
    transform: translateY(-10px);
  }
  100% {
    transform: translateY(0px);
  }
}

.empty-text {
  font-size: 16px;
  color: #8c8c8c;
  margin-bottom: 24px;
}

.option-row {
  display: flex;
  margin-bottom: 16px;
}

.checkbox-group {
  display: flex;
  flex-direction: column;
}

.option-checkbox {
  margin-bottom: 16px;
  transition: all 0.3s;
}

.option-checkbox:hover {
  opacity: 0.8;
}

.checkbox-label {
  display: flex;
  align-items: center;
  font-size: 15px;
}

.delete-confirm {
  display: flex;
  align-items: flex-start;
  padding: 8px 0;
}

.warning-icon {
  color: #ff4d4f;
  font-size: 24px;
  margin-right: 16px;
  margin-top: 2px;
  animation: beat 1.5s ease infinite;
}

@keyframes beat {
  0%, 100% {
    transform: scale(1);
  }
  25% {
    transform: scale(1.1);
  }
}

.delete-message {
  flex: 1;
}

.delete-title {
  margin: 0 0 12px 0;
  font-weight: 500;
  font-size: 16px;
}

.delete-desc {
  margin: 0;
  color: #666;
}

.download-modal :deep(.ant-form-item) {
  margin-bottom: 20px;
}

.form-extra {
  color: #8c8c8c;
  font-size: 13px;
  transition: color 0.3s;
}

.paste-button {
  cursor: pointer;
  color: #1890ff;
  transition: all 0.3s;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 10px;
  border-radius: 4px;
  height: 100%;
  font-weight: 500;
  font-size: 13px;
  gap: 6px;
  position: relative;
  overflow: hidden;
  min-width: 64px;
  box-shadow: none;
}

.paste-button:hover {
  color: #ffffff;
  background-color: #1890ff;
  transform: scale(1.05);
  box-shadow: 0 2px 6px rgba(24, 144, 255, 0.3);
}

.paste-button:active {
  transform: scale(0.98);
  box-shadow: 0 0 0 3px rgba(24, 144, 255, 0.2);
}

.paste-button::after {
  content: "";
  display: block;
  position: absolute;
  width: 100%;
  height: 100%;
  top: 0;
  left: 0;
  pointer-events: none;
  background-image: radial-gradient(circle, #fff 10%, transparent 10.01%);
  background-repeat: no-repeat;
  background-position: 50%;
  transform: scale(10, 10);
  opacity: 0;
  transition: transform 0.5s, opacity 0.5s;
}

.paste-button:active::after {
  transform: scale(0, 0);
  opacity: 0.3;
  transition: 0s;
}

.paste-button svg {
  transition: all 0.3s;
}

.paste-button:hover svg path {
  fill: #ffffff;
}

/* 文件夹选择按钮样式 */
.browse-button {
  cursor: pointer;
  color: #722ed1;
  transition: all 0.3s;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 10px;
  border-radius: 4px;
  height: 100%;
  font-weight: 500;
  font-size: 13px;
  gap: 6px;
  position: relative;
  overflow: hidden;
  min-width: 64px;
  box-shadow: none;
}

.browse-button:hover {
  color: #ffffff;
  background-color: #722ed1;
  transform: scale(1.05);
  box-shadow: 0 2px 6px rgba(114, 46, 209, 0.3);
}

.browse-button:active {
  transform: scale(0.98);
  box-shadow: 0 0 0 3px rgba(114, 46, 209, 0.2);
}

.input-with-effect {
  transition: all 0.3s;
}

.input-with-effect:hover, 
.input-with-effect:focus {
  box-shadow: 0 0 0 2px rgba(24, 144, 255, 0.1);
}

.btn-with-effect {
  position: relative;
  overflow: hidden;
  transition: all 0.3s ease;
}

.btn-with-effect:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.btn-with-effect:after {
  content: "";
  position: absolute;
  top: 50%;
  left: 50%;
  width: 5px;
  height: 5px;
  background: rgba(255, 255, 255, 0.5);
  opacity: 0;
  border-radius: 100%;
  transform: scale(1, 1) translate(-50%);
  transform-origin: 50% 50%;
}

.btn-with-effect:focus:not(:active)::after {
  animation: ripple 1s ease-out;
}

@keyframes ripple {
  0% {
    transform: scale(0, 0);
    opacity: 0.5;
  }
  20% {
    transform: scale(25, 25);
    opacity: 0.3;
  }
  100% {
    opacity: 0;
    transform: scale(40, 40);
  }
}

/* 文件夹选择弹窗样式 */
.folder-tree-modal :deep(.ant-modal-body) {
  padding: 16px;
}

.folder-tree-container {
  max-height: 500px;
  overflow: hidden;
}

.current-path {
  display: flex;
  align-items: center;
  margin-bottom: 16px;
  padding: 12px;
  background-color: #f9f9f9;
  border-radius: 6px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  transition: all 0.3s;
}

.current-path:hover {
  background-color: #f0f7ff;
}

.path-label {
  margin-right: 8px;
  font-weight: 500;
  color: #666;
}

.selected-path {
  font-size: 13px;
  padding: 4px 8px;
  display: flex;
  align-items: center;
  gap: 4px;
  max-width: 400px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  animation: highlight 1s ease-out;
}

.parent-folder-btn {
  margin-left: auto;
  font-size: 12px;
  padding: 0 8px;
  height: 24px;
  display: flex;
  align-items: center;
  border-radius: 12px;
  transition: all 0.3s;
}

.parent-folder-btn:hover {
  background-color: #e6f7ff;
  transform: translateY(-1px);
}

.folder-list-container {
  border: 1px solid #f0f0f0;
  border-radius: 8px;
  padding: 16px;
  max-height: 350px;
  overflow-y: auto;
  background-color: #fafafa;
  box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.05);
  transition: all 0.3s;
  position: relative;
}

.folder-list-container:hover {
  border-color: #d9d9d9;
}

.folder-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  gap: 16px;
  padding: 8px;
}

.folder-item {
  cursor: pointer;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 8px;
  border-radius: 8px;
  transition: all 0.3s;
  border: 1px solid transparent;
  user-select: none;
}

.folder-item:hover {
  background-color: #e6f7ff;
  transform: translateY(-2px);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
}

.folder-selected {
  background-color: #bae7ff;
  border-color: #1890ff;
}

.folder-icon-wrapper {
  margin-bottom: 8px;
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background-color: #e6f7ff;
  transition: all 0.3s;
}

.folder-item:hover .folder-icon-wrapper {
  background-color: #bae7ff;
  transform: scale(1.05);
}

.folder-icon {
  font-size: 24px;
  color: #1890ff;
  transition: all 0.3s;
}

.folder-item:hover .folder-icon {
  color: #40a9ff;
}

.folder-name {
  font-size: 14px;
  font-weight: 500;
  color: #333;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: center;
  transition: all 0.3s;
}

.folder-selected .folder-name {
  color: #1890ff;
}

.empty-folders {
  text-align: center;
  padding: 40px 0;
  color: #999;
  border-radius: 8px;
  background-color: #f9f9f9;
  transition: all 0.3s;
}

.empty-folders:hover {
  background-color: #f0f7ff;
}

.empty-folders .empty-icon {
  font-size: 48px;
  margin-bottom: 12px;
  color: #d9d9d9;
  animation: float 3s ease-in-out infinite;
}

.root-path-info {
  font-size: 12px;
  color: #999;
  margin-top: 10px;
  padding: 4px 8px;
  background-color: #f0f0f0;
  border-radius: 4px;
  display: inline-block;
}

.breadcrumb-container {
  margin-top: 16px;
  padding: 8px 12px;
  background-color: #f9f9f9;
  border-radius: 6px;
  overflow-x: auto;
  white-space: nowrap;
  transition: all 0.3s;
}

.breadcrumb-container:hover {
  background-color: #f0f7ff;
}

.breadcrumb-container a {
  transition: all 0.3s;
  padding: 2px 4px;
  border-radius: 4px;
}

.breadcrumb-container a:hover {
  background-color: #e6f7ff;
  color: #1890ff;
}

.active-crumb {
  color: #1890ff;
  font-weight: 500;
  background-color: #e6f7ff;
}

@keyframes highlight {
  0% { background-color: #e6f7ff; }
  100% { background-color: #1890ff; }
}

/* 添加双击动画效果 */
@keyframes folder-double-click {
  0% { transform: scale(1); }
  50% { transform: scale(0.95); }
  100% { transform: scale(1); }
}

.folder-item:active {
  animation: folder-double-click 0.2s ease;
}

.tree-actions {
  margin-top: 16px;
  text-align: right;
  padding-top: 12px;
  border-top: 1px solid #f0f0f0;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.tree-actions .ant-btn {
  transition: all 0.3s;
  border-radius: 6px;
  display: flex;
  align-items: center;
  gap: 4px;
}

.tree-actions .ant-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

/* 响应式设计优化 */
@media (max-width: 768px) {
  .download-manager {
    padding: 0;
  }
  
  .main-card {
    margin: 0;
    border-radius: 8px;
  }
  
  .main-card :deep(.ant-card-body) {
    padding: 15px !important;
  }
  
  .header {
    flex-direction: column;
  }
  
  .action-buttons {
    margin-top: 12px;
    width: 100%;
    justify-content: space-between;
  }
  
  .task-card :deep(.ant-card-body) {
    padding: 12px !important;
  }
  
  .folder-tree-modal {
    :deep(.ant-modal) {
      max-width: 95vw !important;
      margin: 0 auto;
      top: 20px;
    }
  }
  
  .folder-list-container {
    max-height: 250px;
  }
}

/* 超大屏幕适配 */
@media (min-width: 2560px) {
  .card-row {
    max-width: 2560px;
  }
}

.retry-button {
  background-color: #faad14;
}

.form-section {
  background-color: #fafafa;
  padding: 16px;
  border-radius: 8px;
  margin-bottom: 16px;
  transition: background-color 0.3s;
}

.form-section:hover {
  background-color: #f0f7ff;
}

.section-icon {
  color: #1890ff;
  margin-right: 8px;
}

.download-info {
  margin-top: 16px;
}

.full-page-loading {
  width: 100%;
  display: flex;
  justify-content: center;
}

.full-page-loading :deep(.ant-spin-container) {
  width: 100%;
}

.back-top-button {
  height: 40px;
  width: 40px;
  background-color: #1890ff;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 3px 6px rgba(0, 0, 0, 0.16);
  transition: all 0.3s;
}

.back-top-button:hover {
  background-color: #40a9ff;
  transform: scale(1.1);
}

.download-speed {
  display: flex;
  align-items: center;
  margin-bottom: 8px;
  font-size: 14px;
  color: #1890ff;
  font-weight: 500;
}

.speed-icon {
  margin-right: 6px;
  color: #1890ff;
  animation: flash 1.5s infinite;
}

.speed-text {
  font-family: 'Courier New', monospace;
  margin-right: 8px;
}

.speed-tag {
  font-size: 12px;
  line-height: 14px;
  height: 20px;
  padding: 0 6px;
  border-radius: 10px;
}

@keyframes flash {
  0%, 100% {
    opacity: 1;
  }
  50% {
    opacity: 0.5;
  }
}

.pause-button {
  background-color: #fa8c16;
}

.resume-button {
  background-color: #52c41a;
}

.limit-tag {
  margin-left: 8px;
  font-size: 12px;
}

.global-limit-tag {
  margin-right: 12px;
  font-size: 13px;
  padding: 4px 10px;
  height: auto;
  border-radius: 4px;
  display: flex;
  align-items: center;
  gap: 5px;
  font-weight: 600;
}

.global-limit-tag :deep(.anticon) {
  font-size: 14px;
}

.download-size {
  display: flex;
  align-items: center;
  margin-bottom: 8px;
  font-size: 14px;
  font-weight: 500;
}

.size-icon {
  margin-right: 6px;
}

.size-downloading {
  color: #722ed1;
}

.size-downloading .size-icon {
  color: #722ed1;
  animation: pulse 2s infinite;
}

.size-success {
  color: #52c41a;
}

.size-success .size-icon {
  color: #52c41a;
}

.size-failed {
  color: #f5222d;
}

.size-failed .size-icon {
  color: #f5222d;
}

.size-text {
  font-family: 'Courier New', monospace;
  margin-right: 8px;
}

.size-tag {
  font-size: 12px;
  line-height: 14px;
  height: 20px;
  padding: 0 6px;
  border-radius: 10px;
  margin-left: 4px;
}
</style> 