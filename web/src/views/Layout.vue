<template>
  <div class="layout-container">
    <!-- 移动端遮罩 -->
    <div 
      v-if="sidebarOpen" 
      class="sidebar-overlay"
      @click="sidebarOpen = false"
    ></div>

    <!-- 侧边栏 -->
    <aside class="sidebar" :class="{ open: sidebarOpen }">
      <div class="sidebar-header">
        <div class="logo">
          <div class="logo-icon">
            <el-icon :size="20"><Box /></el-icon>
          </div>
          <span class="logo-text">SSS</span>
        </div>
        <button class="close-btn" @click="sidebarOpen = false">
          <el-icon><Close /></el-icon>
        </button>
      </div>

      <nav class="sidebar-nav">
        <router-link
          v-for="item in menuItems"
          :key="item.name"
          :to="{ name: item.name }"
          class="nav-item"
          :class="{ active: isActive(item.name) }"
          @click="sidebarOpen = false"
        >
          <el-icon :size="18"><component :is="item.icon" /></el-icon>
          <span>{{ item.label }}</span>
        </router-link>
      </nav>

      <div class="sidebar-footer">
        <div class="user-info">
          <el-icon :size="16"><User /></el-icon>
          <span>Admin</span>
        </div>
        <button class="logout-btn" @click="handleLogout">
          <el-icon :size="16"><SwitchButton /></el-icon>
          <span>{{ t('layout.logout') }}</span>
        </button>
      </div>
    </aside>

    <!-- 主内容区 -->
    <div class="main-container">
      <!-- 移动端顶栏 -->
      <header class="mobile-header">
        <button class="menu-btn" @click="sidebarOpen = true">
          <el-icon :size="22"><Menu /></el-icon>
        </button>
        <span class="page-title">{{ currentPageTitle }}</span>
        <div class="header-spacer"></div>
      </header>

      <!-- 内容区 -->
      <main class="main-content">
        <router-view />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../stores/auth'
import {
  Folder, Key, Tools, User, SwitchButton, DataAnalysis, List,
  Menu, Close, Box, Setting
} from '@element-plus/icons-vue'
import axios from 'axios'

const { t } = useI18n()

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const sidebarOpen = ref(false)

const menuItemsConfig = [
  { name: 'Dashboard', labelKey: 'layout.dashboard', icon: DataAnalysis },
  { name: 'Buckets', labelKey: 'layout.buckets', icon: Folder },
  { name: 'ApiKeys', labelKey: 'layout.apiKeys', icon: Key },
  { name: 'Tools', labelKey: 'layout.tools', icon: Tools },
  { name: 'AuditLogs', labelKey: 'layout.auditLogs', icon: List },
  { name: 'Settings', labelKey: 'layout.settings', icon: Setting }
]

const menuItems = computed(() =>
  menuItemsConfig.map(item => ({
    ...item,
    label: t(item.labelKey)
  }))
)

const currentPageTitle = computed(() => {
  const item = menuItems.value.find(m => isActive(m.name))
  return item?.label || 'SSS'
})

function isActive(name: string): boolean {
  if (name === 'Buckets') {
    return route.name === 'Buckets' || route.name === 'Objects'
  }
  return route.name === name
}

async function handleLogout() {
  try {
    await axios.post(`${auth.endpoint}/api/admin/logout`, {}, {
      headers: auth.getAdminHeaders()
    })
  } catch {
    // Ignore logout errors
  }
  auth.logout()
  router.push('/login')
}
</script>

<style scoped>
/* 布局容器 */
.layout-container {
  display: flex;
  min-height: 100vh;
  background: #f8f9fa;
}

/* 侧边栏遮罩 */
.sidebar-overlay {
  display: none;
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  z-index: 998;
}

/* 侧边栏 */
.sidebar {
  width: 220px;
  background: #ffffff;
  border-right: 1px solid #eee;
  display: flex;
  flex-direction: column;
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  z-index: 999;
  transition: transform 0.3s ease;
}

.sidebar-header {
  padding: 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid #f0f0f0;
}

.logo {
  display: flex;
  align-items: center;
  gap: 10px;
}

.logo-icon {
  width: 32px;
  height: 32px;
  background: linear-gradient(135deg, #e67e22 0%, #d35400 100%);
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
}

.logo-text {
  font-size: 18px;
  font-weight: 700;
  color: #333;
  letter-spacing: 1px;
}

.close-btn {
  display: none;
  background: none;
  border: none;
  padding: 8px;
  cursor: pointer;
  color: #666;
  border-radius: 6px;
}

.close-btn:hover {
  background: #f5f5f5;
}

/* 导航菜单 */
.sidebar-nav {
  flex: 1;
  padding: 12px;
  overflow-y: auto;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  margin-bottom: 4px;
  border-radius: 8px;
  color: #555;
  text-decoration: none;
  font-size: 14px;
  font-weight: 500;
  transition: all 0.2s ease;
}

.nav-item:hover {
  background: #fff5f0;
  color: #e67e22;
}

.nav-item.active {
  background: #e67e22;
  color: #ffffff;
}

/* 侧边栏底部 */
.sidebar-footer {
  padding: 16px;
  border-top: 1px solid #f0f0f0;
}

.user-info {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  color: #666;
  font-size: 13px;
  margin-bottom: 8px;
}

.logout-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 10px 12px;
  background: #f8f9fa;
  border: none;
  border-radius: 6px;
  color: #666;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.logout-btn:hover {
  background: #fee2e2;
  color: #dc2626;
}

/* 主内容区 */
.main-container {
  flex: 1;
  margin-left: 220px;
  display: flex;
  flex-direction: column;
  min-height: 100vh;
}

/* 移动端顶栏 */
.mobile-header {
  display: none;
  align-items: center;
  padding: 12px 16px;
  background: #ffffff;
  border-bottom: 1px solid #eee;
  position: sticky;
  top: 0;
  z-index: 100;
}

.menu-btn {
  background: none;
  border: none;
  padding: 8px;
  cursor: pointer;
  color: #333;
  border-radius: 6px;
}

.menu-btn:hover {
  background: #f5f5f5;
}

.page-title {
  flex: 1;
  text-align: center;
  font-size: 16px;
  font-weight: 600;
  color: #333;
}

.header-spacer {
  width: 38px;
}

/* 内容区域 */
.main-content {
  flex: 1;
  padding: 24px;
}

/* 移动端响应式 */
@media (max-width: 768px) {
  .sidebar {
    transform: translateX(-100%);
  }

  .sidebar.open {
    transform: translateX(0);
  }

  .sidebar-overlay {
    display: block;
  }

  .close-btn {
    display: block;
  }

  .main-container {
    margin-left: 0;
  }

  .mobile-header {
    display: flex;
  }

  .main-content {
    padding: 16px;
  }
}

/* 平板响应式 */
@media (max-width: 1024px) and (min-width: 769px) {
  .sidebar {
    width: 200px;
  }

  .main-container {
    margin-left: 200px;
  }

  .main-content {
    padding: 20px;
  }
}
</style>
