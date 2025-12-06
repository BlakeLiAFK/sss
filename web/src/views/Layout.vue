<template>
  <el-container class="layout-container">
    <el-header class="header">
      <div class="logo">
        <el-icon :size="28" color="#fff"><Box /></el-icon>
        <span>SSS</span>
      </div>
      <el-menu mode="horizontal" :default-active="route.name as string" router background-color="#409EFF" text-color="#fff" active-text-color="#ffd04b">
        <el-menu-item index="Buckets" :route="{ name: 'Buckets' }">
          <el-icon><Folder /></el-icon>
          Buckets
        </el-menu-item>
        <el-menu-item index="ApiKeys" :route="{ name: 'ApiKeys' }">
          <el-icon><Key /></el-icon>
          API Keys
        </el-menu-item>
        <el-menu-item index="Tools" :route="{ name: 'Tools' }">
          <el-icon><Tools /></el-icon>
          Tools
        </el-menu-item>
      </el-menu>
      <div class="user-info">
        <el-dropdown @command="handleCommand">
          <span class="el-dropdown-link">
            Admin
            <el-icon class="el-icon--right"><ArrowDown /></el-icon>
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="logout">Logout</el-dropdown-item>
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
import axios from 'axios'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

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
}

.header {
  display: flex;
  align-items: center;
  background-color: #409EFF;
  padding: 0 20px;
}

.logo {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #fff;
  font-size: 20px;
  font-weight: 600;
  margin-right: 40px;
}

.el-menu {
  flex: 1;
  border-bottom: none !important;
}

.user-info {
  color: #fff;
}

.el-dropdown-link {
  color: #fff;
  cursor: pointer;
  display: flex;
  align-items: center;
}

.main {
  background-color: #f5f7fa;
  padding: 20px;
}
</style>
