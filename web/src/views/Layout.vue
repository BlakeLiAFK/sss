<template>
  <el-container class="layout-container">
    <el-header class="header">
      <div class="header-left">
        <div class="logo">
          <div class="logo-icon">
            <el-icon :size="22"><Box /></el-icon>
          </div>
          <span class="logo-text">SSS</span>
        </div>
        <nav class="nav-menu">
          <router-link
            v-for="item in menuItems"
            :key="item.name"
            :to="{ name: item.name }"
            class="nav-item"
            :class="{ active: route.name === item.name }"
          >
            <el-icon><component :is="item.icon" /></el-icon>
            <span>{{ item.label }}</span>
          </router-link>
        </nav>
      </div>
      <div class="header-right">
        <el-dropdown @command="handleCommand" trigger="click">
          <div class="user-avatar">
            <el-icon :size="18"><User /></el-icon>
          </div>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item disabled>
                <el-icon><User /></el-icon>
                Admin
              </el-dropdown-item>
              <el-dropdown-item divided command="logout">
                <el-icon><SwitchButton /></el-icon>
                Logout
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </el-header>
    <el-main class="main">
      <router-view />
    </el-main>
  </el-container>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { Folder, Key, Tools, User, SwitchButton } from '@element-plus/icons-vue'
import axios from 'axios'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const menuItems = [
  { name: 'Buckets', label: 'Buckets', icon: Folder },
  { name: 'ApiKeys', label: 'API Keys', icon: Key },
  { name: 'Tools', label: 'Tools', icon: Tools }
]

async function handleCommand(cmd: string) {
  if (cmd === 'logout') {
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
}
</script>

<style scoped>
.layout-container {
  min-height: 100vh;
  background: #f1f5f9;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #ffffff;
  padding: 0 24px;
  height: 60px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
  position: sticky;
  top: 0;
  z-index: 100;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 48px;
}

.logo {
  display: flex;
  align-items: center;
  gap: 10px;
}

.logo-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%);
  border-radius: 8px;
  color: #ffffff;
}

.logo-text {
  font-size: 20px;
  font-weight: 700;
  color: #1e293b;
  letter-spacing: 1px;
}

.nav-menu {
  display: flex;
  gap: 4px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 6px;
  color: #64748b;
  text-decoration: none;
  font-size: 14px;
  font-weight: 500;
  transition: all 0.2s ease;
}

.nav-item:hover {
  color: #3b82f6;
  background: #f1f5f9;
}

.nav-item.active {
  color: #3b82f6;
  background: #eff6ff;
}

.header-right {
  display: flex;
  align-items: center;
}

.user-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  background: #f1f5f9;
  border-radius: 50%;
  color: #64748b;
  cursor: pointer;
  transition: all 0.2s ease;
}

.user-avatar:hover {
  background: #e2e8f0;
  color: #475569;
}

.main {
  padding: 24px;
  background: #f1f5f9;
}
</style>
