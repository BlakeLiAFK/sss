import axios from 'axios'
import { useAuthStore } from '../stores/auth'

/**
 * 获取 API 基础 URL
 * 优先级：auth.endpoint > VITE_BASE_URL > 当前域名
 */
export function getBaseUrl(): string {
  // 尝试从 auth store 获取（用户已登录后）
  try {
    const auth = useAuthStore()
    if (auth.endpoint) {
      return auth.endpoint
    }
  } catch {
    // Pinia 还未初始化，忽略
  }
  
  // 使用环境变量或当前域名
  return import.meta.env.VITE_BASE_URL || window.location.origin
}

/**
 * 创建配置了 baseURL 的 axios 实例
 */
export const apiClient = axios.create({
  timeout: 30000
})

// 请求拦截器：动态设置 baseURL
apiClient.interceptors.request.use((config) => {
  // 如果 URL 已经是完整路径，不做处理
  if (config.url?.startsWith('http')) {
    return config
  }
  
  // 设置 baseURL
  config.baseURL = getBaseUrl()
  return config
})

/**
 * 管理 API 客户端（需要认证）
 */
export function adminApi() {
  const auth = useAuthStore()
  return {
    get: (url: string, config?: any) => apiClient.get(url, {
      ...config,
      headers: {
        ...config?.headers,
        'X-Admin-Token': auth.adminToken,
        'Content-Type': 'application/json'
      }
    }),
    post: (url: string, data?: any, config?: any) => apiClient.post(url, data, {
      ...config,
      headers: {
        ...config?.headers,
        'X-Admin-Token': auth.adminToken,
        'Content-Type': 'application/json'
      }
    }),
    put: (url: string, data?: any, config?: any) => apiClient.put(url, data, {
      ...config,
      headers: {
        ...config?.headers,
        'X-Admin-Token': auth.adminToken,
        'Content-Type': 'application/json'
      }
    }),
    delete: (url: string, config?: any) => apiClient.delete(url, {
      ...config,
      headers: {
        ...config?.headers,
        'X-Admin-Token': auth.adminToken,
        'Content-Type': 'application/json'
      }
    })
  }
}

export default apiClient
