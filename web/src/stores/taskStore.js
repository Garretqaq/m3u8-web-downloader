import { defineStore } from 'pinia'
import axios from 'axios'

export const useTaskStore = defineStore('task', {
  state: () => ({
    tasks: [],
    loading: false,
    refreshing: false,
    retryCount: 0,
    initialLoading: false
  }),
  
  getters: {
    sortedTasks: (state) => {
      return [...state.tasks].sort((a, b) => {
        // 优先显示下载中的任务
        if (a.status === 'downloading' && b.status !== 'downloading') return -1
        if (a.status !== 'downloading' && b.status === 'downloading') return 1
        // 然后是正在转换格式的任务
        if (a.status === 'converting' && b.status !== 'converting') return -1
        if (a.status !== 'converting' && b.status === 'converting') return 1
        // 然后是等待下载的任务
        if (a.status === 'pending' && b.status !== 'pending') return -1
        if (a.status !== 'pending' && b.status === 'pending') return 1
        // 状态相同时，按创建时间倒序排序（新创建的排在前面）
        return b.created - a.created
      })
    }
  },
  
  actions: {
    async fetchTasks() {
      try {
        this.refreshing = true
        const response = await axios.get('/api/tasks')
        if (response.data.success) {
          this.tasks = response.data.data || []
          // 重置重试计数
          this.retryCount = 0
        }
      } catch (error) {
        console.error('获取任务列表失败:', error)
        // 如果是网络错误，最多重试3次
        if (this.retryCount < 3) {
          this.retryCount++
          setTimeout(() => this.fetchTasks(), 2000) // 2秒后重试
        }
      } finally {
        this.refreshing = false
      }
    },
    
    async createTask(taskData) {
      try {
        this.loading = true
        console.log('taskStore: 开始创建任务，数据:', taskData)
        
        const downloadData = {
          url: taskData.url,
          output: taskData.output,
          c: taskData.c,
          deleteTs: taskData.deleteTs === undefined ? true : Boolean(taskData.deleteTs),
          convertToMp4: taskData.convertToMp4 === undefined ? true : Boolean(taskData.convertToMp4)
        }
        
        // 如果有自定义文件名，则添加到请求参数中
        if (taskData.customFileName && taskData.customFileName.trim() !== '') {
          downloadData.customFileName = taskData.customFileName.trim()
        }
        
        // 添加避免文件覆盖的选项
        downloadData.avoidOverwrite = true
        
        // 添加分块处理选项，默认启用
        downloadData.chunkProcessing = true
        
        console.log('taskStore: 准备发送API请求，数据:', downloadData)
        
        // 创建一个可以被取消的请求
        const controller = new AbortController()
        const timeoutId = setTimeout(() => controller.abort(), 10000) // 10秒超时
        
        const response = await axios.post('/api/download', downloadData)
        if (response.data.success) {
          await this.fetchTasks()
          return { success: true }
        } else {
          return { success: false, message: response.data.message }
        }
      } catch (error) {
        console.error('创建下载任务失败:', error)
        return { success: false, message: error.response?.data?.message || '创建下载任务失败' }
      } finally {
        this.loading = false
      }
    },
    
    async deleteTask(taskId) {
      try {
        const response = await axios.delete(`/api/tasks/${taskId}`)
        if (response.data.success) {
          await this.fetchTasks()
          return { success: true }
        } else {
          return { success: false, message: response.data.message }
        }
      } catch (error) {
        console.error('删除任务失败:', error)
        return { success: false, message: error.response?.data?.message || '删除任务失败' }
      }
    },
    
    async openFileLocation(taskId) {
      try {
        const response = await axios.post(`/api/tasks/${taskId}/open-location`)
        return { 
          success: response.data.success, 
          message: response.data.message 
        }
      } catch (error) {
        console.error('打开文件位置失败:', error)
        return { 
          success: false, 
          message: error.response?.data?.message || '打开文件位置失败' 
        }
      }
    },
    
    async retryTask(taskId) {
      try {
        const response = await axios.post(`/api/tasks/${taskId}/retry`)
        if (response.data.success) {
          await this.fetchTasks()
          return { success: true }
        } else {
          return { success: false, message: response.data.message }
        }
      } catch (error) {
        console.error('重试下载任务失败:', error)
        return { success: false, message: error.response?.data?.message || '重试下载任务失败' }
      }
    },
    
    async pauseTask(taskId) {
      try {
        const response = await axios.post(`/api/tasks/${taskId}/pause`)
        if (response.data.success) {
          await this.fetchTasks()
          return { success: true }
        } else {
          return { success: false, message: response.data.message }
        }
      } catch (error) {
        console.error('暂停任务失败:', error)
        return { success: false, message: error.response?.data?.message || '暂停任务失败' }
      }
    },
    
    async resumeTask(taskId) {
      try {
        const response = await axios.post(`/api/tasks/${taskId}/resume`)
        if (response.data.success) {
          await this.fetchTasks()
          return { success: true }
        } else {
          return { success: false, message: response.data.message }
        }
      } catch (error) {
        console.error('继续任务失败:', error)
        return { success: false, message: error.response?.data?.message || '继续任务失败' }
      }
    }
  }
}) 