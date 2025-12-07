import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  // 管理员 session token
  const adminToken = ref(localStorage.getItem('adminToken') || '')

  // S3 API 配置（优先使用环境变量，其次 localStorage，最后当前域名）
  const endpoint = ref(localStorage.getItem('endpoint') || import.meta.env.VITE_BASE_URL || window.location.origin)
  const region = ref(localStorage.getItem('region') || 'us-east-1')

  // S3 凭证（用于 AWS Signature V4 签名）
  const accessKeyId = ref(localStorage.getItem('accessKeyId') || '')
  const secretAccessKey = ref(localStorage.getItem('secretAccessKey') || '')

  const isLoggedIn = computed(() => adminToken.value !== '')

  // 管理员登录
  // 注意：必须先执行 localStorage 操作，再更新响应式状态
  // 因为响应式赋值会触发 isLoggedIn computed，可能导致路由跳转中断后续代码
  function login(token: string, ep: string, reg: string, akId: string, skKey: string) {
    // 1. 先持久化到 localStorage（同步操作，无副作用）
    localStorage.setItem('adminToken', token)
    localStorage.setItem('endpoint', ep)
    localStorage.setItem('region', reg)
    localStorage.setItem('accessKeyId', akId)
    localStorage.setItem('secretAccessKey', skKey)

    // 2. 再更新响应式状态（可能触发 watcher 和导航）
    adminToken.value = token
    endpoint.value = ep
    region.value = reg
    accessKeyId.value = akId
    secretAccessKey.value = skKey
  }

  // 管理员登出
  function logout() {
    adminToken.value = ''
    accessKeyId.value = ''
    secretAccessKey.value = ''

    localStorage.removeItem('adminToken')
    localStorage.removeItem('accessKeyId')
    localStorage.removeItem('secretAccessKey')
  }

  // 获取管理员请求头
  function getAdminHeaders() {
    return {
      'X-Admin-Token': adminToken.value,
      'Content-Type': 'application/json'
    }
  }

  return {
    adminToken,
    endpoint,
    region,
    accessKeyId,
    secretAccessKey,
    isLoggedIn,
    login,
    logout,
    getAdminHeaders
  }
})
