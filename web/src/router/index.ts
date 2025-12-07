import { createRouter, createWebHashHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import apiClient from '../api/client'

// 系统安装状态缓存
let systemInstalled: boolean | null = null

// 检查系统安装状态
async function checkSystemInstalled(): Promise<boolean> {
  if (systemInstalled !== null) {
    return systemInstalled
  }
  try {
    const response = await apiClient.get('/api/setup/status')
    systemInstalled = response.data.installed === true
    return systemInstalled
  } catch (e) {
    // 如果检测失败，假设已安装（避免阻塞）
    console.error('检测安装状态失败:', e)
    return true
  }
}

// 重置安装状态缓存（安装完成后调用）
export function resetInstallStatus() {
  systemInstalled = null
}

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: '/setup',
      name: 'Setup',
      component: () => import('../views/Setup.vue'),
      meta: { requiresSetup: true }
    },
    {
      path: '/login',
      name: 'Login',
      component: () => import('../views/Login.vue')
    },
    {
      path: '/',
      component: () => import('../views/Layout.vue'),
      children: [
        {
          path: '',
          name: 'Dashboard',
          component: () => import('../views/Dashboard.vue')
        },
        {
          path: 'buckets',
          name: 'Buckets',
          component: () => import('../views/Buckets.vue')
        },
        {
          path: 'bucket/:name',
          name: 'Objects',
          component: () => import('../views/Objects.vue')
        },
        {
          path: 'tools',
          name: 'Tools',
          component: () => import('../views/Tools.vue')
        },
        {
          path: 'apikeys',
          name: 'ApiKeys',
          component: () => import('../views/ApiKeys.vue')
        },
        {
          path: 'audit',
          name: 'AuditLogs',
          component: () => import('../views/AuditLogs.vue')
        },
        {
          path: 'settings',
          name: 'Settings',
          component: () => import('../views/Settings.vue')
        }
      ]
    }
  ]
})

router.beforeEach(async (to, _from, next) => {
  const auth = useAuthStore()
  
  // 检查系统是否已安装
  const installed = await checkSystemInstalled()
  
  // 如果未安装且不是安装页面，跳转到安装页面
  if (!installed && to.name !== 'Setup') {
    next({ name: 'Setup' })
    return
  }
  
  // 如果已安装且访问安装页面，跳转到登录页
  if (installed && to.name === 'Setup') {
    next({ name: 'Login' })
    return
  }
  
  // 登录检查（跳过安装页面和登录页面）
  if (to.name !== 'Login' && to.name !== 'Setup' && !auth.isLoggedIn) {
    next({ name: 'Login' })
    return
  }
  
  next()
})

export default router
