import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  // 管理员 session token
  const adminToken = ref(localStorage.getItem('adminToken') || '')

  // S3 API 配置（用于预签名 URL 等）
  const endpoint = ref(localStorage.getItem('endpoint') || window.location.origin)
  const region = ref(localStorage.getItem('region') || 'us-east-1')

  const isLoggedIn = computed(() => adminToken.value !== '')

  // 管理员登录
  function login(token: string, ep: string, reg: string) {
    adminToken.value = token
    endpoint.value = ep
    region.value = reg
    localStorage.setItem('adminToken', token)
    localStorage.setItem('endpoint', ep)
    localStorage.setItem('region', reg)
  }

  // 管理员登出
  function logout() {
    adminToken.value = ''
    localStorage.removeItem('adminToken')
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
    isLoggedIn,
    login,
    logout,
    getAdminHeaders
  }
})
