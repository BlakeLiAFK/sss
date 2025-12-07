import axios from 'axios'
import { useAuthStore } from '../stores/auth'
import router from '../router'

// 配置 axios 响应拦截器
// 当收到 401 响应时自动登出并跳转到登录页
export function setupAxiosInterceptor() {
  axios.interceptors.response.use(
    (response) => response,
    (error) => {
      if (error.response?.status === 401) {
        const auth = useAuthStore()
        auth.logout()
        router.push({ name: 'Login' })
      }
      return Promise.reject(error)
    }
  )
}
