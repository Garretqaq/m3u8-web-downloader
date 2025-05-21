<template>
  <div class="settings-container animate__animated animate__fadeIn animate__faster">
    <a-card class="settings-card" :body-style="{ padding: 0 }">
      <!-- 页面标题区域 -->
      <div class="header-section">
        <a-typography-title :level="4" style="margin: 0; color: #1890ff;">
          <SettingOutlined class="title-icon" /> 配置设置
        </a-typography-title>
        <a-typography-paragraph style="margin: 8px 0 0 0; color: #666; font-size: 14px;">
          设置默认下载参数，让每次下载更加便捷
        </a-typography-paragraph>
      </div>
      
      <!-- 表单区域 -->
      <div class="form-container">
        <a-form 
          :model="formState" 
          layout="vertical"
          @finish="saveSettings"
          ref="formRef"
        >
          <div class="settings-content">
            <div class="settings-left">
              <!-- 基本设置区块 -->
              <div class="settings-section animate__animated animate__fadeIn animate__faster">
                <div class="section-header">
                  <div class="section-title">
                    <FolderOutlined class="section-icon" />
                    <span>存储设置</span>
                  </div>
                </div>
                
                <a-form-item 
                  name="defaultOutputPath" 
                  label="默认下载位置" 
                  :rules="[{ required: true, whitespace: true, message: '请输入默认下载保存目录' }]"
                >
                  <div class="input-row">
                    <a-input 
                      v-model:value="formState.defaultOutputPath" 
                      placeholder="如 /data/videos" 
                      size="middle"
                      :prefix="h(FolderOutlined)"
                      class="input-with-effect"
                      ref="defaultOutputPathInputRef"
                    >
                      <template #addonAfter>
                        <span
                          class="paste-button"
                          title="粘贴剪贴板"
                          @click="pasteOutputPath"
                        >
                          <svg width="14" height="14" viewBox="0 0 1024 1024">
                            <path d="M832 112H724V72c0-22.1-17.9-40-40-40H340c-22.1 0-40 17.9-40 40v40H192c-35.3 0-64 28.7-64 64v712c0 35.3 28.7 64 64 64h640c35.3 0 64-28.7 64-64V176c0-35.3-28.7-64-64-64zM372 80h280v32H372V80zm484 808c0 17.7-14.3 32-32 32H200c-17.7 0-32-14.3-32-32V184c0-17.7 14.3-32 32-32h88v40c0 22.1 17.9 40 40 40h304c22.1 0 40-17.9 40-40v-40h88c17.7 0 32 14.3 32 32v704z" fill="#1890ff"/>
                          </svg>
                          <span>粘贴</span>
                        </span>
                      </template>
                    </a-input>
                  </div>
                  <div class="form-extra">请确保目录已存在且有写入权限</div>
                </a-form-item>
              </div>
              
              <!-- 下载设置区块 -->
              <div class="settings-section animate__animated animate__fadeIn animate__faster download-settings-section">
                <div class="section-header">
                  <div class="section-title">
                    <ThunderboltOutlined class="section-icon" />
                    <span>下载设置</span>
                  </div>
                </div>
                
                <a-form-item 
                  name="defaultThreadCount" 
                  label="默认下载线程" 
                  :rules="[{ required: true, type: 'number', min: 1, max: 128, message: '线程数需为1-128' }]"
                >
                  <div class="thread-row">
                    <a-slider
                      v-model:value="formState.defaultThreadCount"
                      :min="1"
                      :max="128"
                      :marks="{ 25: '', 50: '', 75: '', 100: '' }"
                      class="thread-slider"
                    />
                    <a-input-number
                      v-model:value="formState.defaultThreadCount"
                      :min="1"
                      :max="128"
                      style="width: 70px;"
                      size="middle"
                      class="thread-input"
                    />
                  </div>
                  <div class="form-extra">建议10-50，过高可能影响稳定性</div>
                </a-form-item>

                <a-form-item 
                  name="maxConcurrentDownload" 
                  label="同时下载数量" 
                  :rules="[{ required: true, type: 'number', min: 1, max: 10, message: '同时下载数量为1-10' }]"
                >
                  <div class="thread-row">
                    <a-slider
                      v-model:value="formState.maxConcurrentDownload"
                      :min="1"
                      :max="10"
                      :marks="{ 1: '', 3: '', 5: '', 10: '' }"
                      class="thread-slider"
                    />
                    <a-input-number
                      v-model:value="formState.maxConcurrentDownload"
                      :min="1"
                      :max="10"
                      style="width: 70px;"
                      size="middle"
                      class="thread-input"
                    />
                  </div>
                  <div class="form-extra">同时进行下载的最大任务数量，超出此数量的任务将会排队等待</div>
                </a-form-item>
              </div>
            </div>
            
            <div class="settings-right">
              <!-- 格式设置区块 -->
              <div class="settings-section animate__animated animate__fadeIn animate__faster">
                <div class="section-header">
                  <div class="section-title">
                    <FileOutlined class="section-icon" />
                    <span>格式设置</span>
                  </div>
                </div>
                
                <div class="option-cards">
                  <a-card 
                    class="option-card" 
                    :class="{ 'option-selected': formState.defaultConvertToMp4 }"
                    hoverable 
                    @click="formState.defaultConvertToMp4 = !formState.defaultConvertToMp4"
                  >
                    <VideoCameraOutlined class="option-icon" />
                    <div class="option-content">
                      <div class="option-title">MP4格式</div>
                      <div class="option-desc">默认下载为MP4格式</div>
                      <a-switch v-model:checked="formState.defaultConvertToMp4" size="small" />
                    </div>
                  </a-card>
                  
                  <a-card 
                    class="option-card" 
                    :class="{ 'option-selected': formState.defaultDeleteTs }"
                    hoverable 
                    @click="formState.defaultDeleteTs = !formState.defaultDeleteTs"
                  >
                    <DeleteOutlined class="option-icon" />
                    <div class="option-content">
                      <div class="option-title">删除分片</div>
                      <div class="option-desc">默认合并后删除分片</div>
                      <a-switch v-model:checked="formState.defaultDeleteTs" size="small" />
                    </div>
                  </a-card>
                </div>
                
                <!-- 保存按钮区域 -->
                <div class="save-section animate__animated animate__fadeIn animate__faster">
                  <a-button 
                    type="primary" 
                    html-type="submit" 
                    size="middle" 
                    :loading="loading"
                    class="save-button btn-with-effect"
                  >
                    <template #icon><SaveOutlined /></template>
                    保存设置
                  </a-button>
                </div>
              </div>
            </div>
          </div>
          
          <!-- 结果提示 -->
          <a-alert 
            v-if="saveResult.show" 
            :type="saveResult.type" 
            :message="saveResult.message" 
            show-icon 
            class="result-message" 
            banner
          />
        </a-form>
      </div>
    </a-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, h, nextTick } from 'vue'
import { 
  FolderOutlined, 
  SaveOutlined, 
  VideoCameraOutlined, 
  DeleteOutlined,
  SettingOutlined,
  ThunderboltOutlined,
  FileOutlined
} from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import axios from 'axios'

const formRef = ref(null)
const defaultOutputPathInputRef = ref(null)
const loading = ref(false)
const saveResult = reactive({
  show: false,
  type: 'success',
  message: ''
})

const formState = reactive({
  defaultOutputPath: '',
  defaultThreadCount: 25,
  defaultConvertToMp4: true,
  defaultDeleteTs: true,
  maxConcurrentDownload: 3
})

// 从服务器加载配置
const loadSettings = async () => {
  try {
    loading.value = true
    const response = await axios.get('/api/settings')
    console.log('加载到的配置:', response.data)
    if (response.data.success) {
      const data = response.data.data || {}
      formState.defaultOutputPath = data.defaultOutputPath || '';
      formState.defaultThreadCount = data.defaultThreadCount || 25;
      formState.defaultConvertToMp4 = data.defaultConvertToMp4 !== undefined ? data.defaultConvertToMp4 : true;
      formState.defaultDeleteTs = data.defaultDeleteTs !== undefined ? data.defaultDeleteTs : true;
      formState.maxConcurrentDownload = data.maxConcurrentDownload || 3;
      console.log('设置后的表单状态:', formState)
    } else {
      message.error('加载配置失败: ' + response.data.message)
    }
  } catch (error) {
    console.error('无法加载配置:', error)
    message.error('无法加载配置，请稍后再试。')
  } finally {
    loading.value = false
  }
}

// 保存配置到服务器
const saveSettings = async () => {
  try {
    loading.value = true
    
    const response = await axios.post('/api/settings', {
      defaultOutputPath: formState.defaultOutputPath,
      defaultThreadCount: formState.defaultThreadCount,
      defaultConvertToMp4: formState.defaultConvertToMp4,
      defaultDeleteTs: formState.defaultDeleteTs,
      maxConcurrentDownload: formState.maxConcurrentDownload
    })
    
    if (response.data.success) {
      saveResult.type = 'success'
      saveResult.message = '配置保存成功!'
      saveResult.show = true
      message.success('配置保存成功!')
      
      // 3秒后隐藏结果消息
      setTimeout(() => {
        saveResult.show = false
      }, 3000)
    } else {
      saveResult.type = 'error'
      saveResult.message = '保存失败: ' + response.data.message
      saveResult.show = true
      message.error('保存失败: ' + response.data.message)
    }
  } catch (error) {
    console.error('保存配置失败:', error)
    saveResult.type = 'error'
    saveResult.message = '无法保存配置，请稍后再试'
    saveResult.show = true
    message.error('无法保存配置，请稍后再试')
  } finally {
    loading.value = false
  }
}

// 粘贴下载路径
const pasteOutputPath = async () => {
  try {
    // 添加按钮动画效果
    const button = document.querySelector('.paste-button');
    button.classList.add('paste-animation');
    setTimeout(() => {
      button.classList.remove('paste-animation');
    }, 500);
    
    const text = await navigator.clipboard.readText()
    formState.defaultOutputPath = text
    message.success('已粘贴剪贴板内容')
  } catch (e) {
    message.error('无法读取剪贴板内容，请检查浏览器权限')
  }
}

// 页面加载时自动获取配置
onMounted(() => {
  // 立即显示页面框架，然后异步加载数据
  nextTick(() => {
    loadSettings()
  })
})
</script>

<style scoped>
@import 'https://cdnjs.cloudflare.com/ajax/libs/animate.css/4.1.1/animate.min.css';

:root {
  --animate-duration: 0.5s; /* 设置全局动画时长为0.5秒，默认是1秒 */
  --animate-delay: 0.1s;    /* 减少默认延迟 */
}

.settings-container {
  width: 100%;
  max-width: 1100px;
  margin: 0 auto;
  padding: 0 16px;
}

.settings-card {
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05), 0 10px 25px rgba(0, 0, 0, 0.07);
  transition: all 0.3s;
}

.header-section {
  background: linear-gradient(135deg, #1890ff 0%, #0050b3 100%);
  color: white;
  padding: 16px 20px;
  border-top-left-radius: 12px;
  border-top-right-radius: 12px;
}

.header-section :deep(.ant-typography) {
  color: white !important;
}

.title-icon {
  margin-right: 8px;
  font-size: 22px;
  animation: spin 10s linear infinite;
}

@keyframes spin {
  100% { transform: rotate(360deg); }
}

.form-container {
  padding: 20px;
  background: white;
}

.settings-content {
  display: flex;
  gap: 20px;
}

.settings-left {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.settings-left .settings-section {
  width: 100%;
  margin-bottom: 20px;
  flex-shrink: 0;
  overflow: visible;
}

.settings-right {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.settings-section {
  margin-bottom: 15px;
  background-color: #fafafa;
  border-radius: 10px;
  padding: 15px;
  transition: all 0.3s;
  height: auto;
  min-height: 100px;
  position: relative;
  z-index: 1;
}

.settings-section + .settings-section {
  margin-top: 15px;
  clear: both;
}

.settings-section:hover {
  background-color: #f0f7ff;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
}

.section-header {
  margin-bottom: 10px;
}

.section-title {
  font-size: 15px;
  font-weight: 500;
  display: flex;
  align-items: center;
  color: #1890ff;
}

.section-icon {
  margin-right: 6px;
  font-size: 16px;
}

.input-with-effect {
  transition: all 0.3s;
  border-radius: 6px;
}

.input-with-effect:hover, 
.input-with-effect:focus {
  box-shadow: 0 0 0 2px rgba(24, 144, 255, 0.1);
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

.thread-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.thread-slider {
  flex: 1;
}

.btn-with-effect {
  position: relative;
  overflow: hidden;
  transition: all 0.3s ease;
  border-radius: 6px;
}

.btn-with-effect:hover {
  transform: translateY(-2px);
  box-shadow: 0 5px 15px rgba(24, 144, 255, 0.2);
}

.form-extra {
  color: #8c8c8c;
  font-size: 12px;
  margin-top: 4px;
}

.save-button {
  min-width: 100px;
  background: linear-gradient(to right, #1890ff, #096dd9);
  border: none;
  margin-top: 15px;
}

.save-button:hover {
  background: linear-gradient(to right, #40a9ff, #1890ff);
}

.result-message {
  margin-top: 15px;
  border-radius: 6px;
  overflow: hidden;
}

.option-cards {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.option-card {
  border-radius: 8px;
  padding: 4px;
  transition: all 0.3s;
  display: flex;
  cursor: pointer;
}

.option-card.option-selected {
  border-color: #1890ff;
  background-color: #e6f7ff;
}

.option-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.12);
}

.option-icon {
  font-size: 24px;
  color: #1890ff;
  margin-right: 12px;
  display: flex;
  align-items: center;
}

.option-content {
  flex: 1;
}

.option-title {
  font-weight: 500;
  margin-bottom: 2px;
  font-size: 14px;
  color: #262626;
}

.option-desc {
  font-size: 12px;
  color: #8c8c8c;
  margin-bottom: 5px;
}

.save-section {
  margin-top: auto;
  display: flex;
  justify-content: flex-end;
}

.download-settings-section {
  border: 2px solid #1890ff;
  background-color: #f0f9ff;
  box-shadow: 0 2px 8px rgba(24, 144, 255, 0.15);
  margin-top: 16px;
  margin-bottom: 16px;
  min-height: 120px;
  padding-top: 15px;
  padding-bottom: 15px;
}

.download-settings-section .section-title {
  font-size: 16px;
  font-weight: 600;
  color: #1890ff;
}

.download-settings-section .thread-slider {
  margin-top: 8px;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .settings-container {
    padding: 0 8px;
  }
  
  .settings-content {
    flex-direction: column;
    gap: 15px;
  }
  
  .option-cards {
    flex-direction: row;
  }
  
  .option-card {
    flex: 1;
  }
  
  .save-section {
    justify-content: center;
    margin-top: 15px;
  }
  
  .thread-row {
    flex-direction: column;
    align-items: stretch;
  }
  
  .thread-input {
    width: 100% !important;
  }
}
</style> 