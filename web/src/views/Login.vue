<template>
  <div class="login-container">
    <div class="login-box">
      <div class="login-header">
        <div class="logo-wrapper">
          <el-icon :size="40"><Box /></el-icon>
        </div>
        <h1 class="title">SSS</h1>
        <p class="subtitle">Simple S3 Server</p>
      </div>

      <el-form :model="form" :rules="rules" ref="formRef" class="login-form">
        <el-form-item prop="endpoint">
          <el-input
            v-model="form.endpoint"
            placeholder="Server Endpoint"
            :prefix-icon="Link"
            size="large"
          />
        </el-form-item>
        <el-form-item prop="region">
          <el-input
            v-model="form.region"
            placeholder="Region"
            :prefix-icon="Location"
            size="large"
          />
        </el-form-item>
        <el-form-item prop="username">
          <el-input
            v-model="form.username"
            placeholder="Username"
            :prefix-icon="User"
            size="large"
          />
        </el-form-item>
        <el-form-item prop="password">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="Password"
            :prefix-icon="Lock"
            size="large"
            show-password
            @keyup.enter="handleLogin"
          />
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            @click="handleLogin"
            :loading="loading"
            size="large"
            class="login-button"
          >
            Sign In
          </el-button>
        </el-form-item>
      </el-form>

      <div class="login-footer">
        <span>Lightweight Enterprise Storage</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance } from 'element-plus'
import { useAuthStore } from '../stores/auth'
import { User, Lock, Link, Location } from '@element-plus/icons-vue'
import axios from 'axios'

const router = useRouter()
const auth = useAuthStore()
const formRef = ref<FormInstance>()
const loading = ref(false)

const form = reactive({
  endpoint: auth.endpoint || window.location.origin,
  region: auth.region || 'us-east-1',
  username: 'admin',
  password: ''
})

const rules = {
  endpoint: [{ required: true, message: 'Please input Endpoint', trigger: 'blur' }],
  username: [{ required: true, message: 'Please input Username', trigger: 'blur' }],
  password: [{ required: true, message: 'Please input Password', trigger: 'blur' }]
}

async function handleLogin() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return

    loading.value = true
    try {
      const response = await axios.post(`${form.endpoint}/api/admin/login`, {
        username: form.username,
        password: form.password
      })

      if (response.data.success) {
        auth.login(response.data.token, form.endpoint, form.region)
        ElMessage.success('Login successful')
        router.push('/')
      } else {
        ElMessage.error(response.data.message || 'Login failed')
      }
    } catch (e: any) {
      ElMessage.error('Login failed: ' + (e.response?.data?.message || e.message))
    } finally {
      loading.value = false
    }
  })
}
</script>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
}

.login-box {
  width: 380px;
  background: #ffffff;
  border-radius: 12px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  padding: 40px;
}

.login-header {
  text-align: center;
  margin-bottom: 32px;
}

.logo-wrapper {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 72px;
  height: 72px;
  background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%);
  border-radius: 16px;
  color: #ffffff;
  margin-bottom: 16px;
}

.title {
  font-size: 28px;
  font-weight: 700;
  color: #1e293b;
  margin: 0;
  letter-spacing: 2px;
}

.subtitle {
  font-size: 14px;
  color: #64748b;
  margin: 8px 0 0;
}

.login-form {
  margin-bottom: 16px;
}

.login-form :deep(.el-input__wrapper) {
  border-radius: 8px;
  box-shadow: 0 0 0 1px #e2e8f0;
}

.login-form :deep(.el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px #94a3b8;
}

.login-form :deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 2px #3b82f6;
}

.login-button {
  width: 100%;
  height: 44px;
  font-size: 16px;
  font-weight: 600;
  border-radius: 8px;
  background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%);
  border: none;
}

.login-button:hover {
  background: linear-gradient(135deg, #2563eb 0%, #1e40af 100%);
}

.login-footer {
  text-align: center;
  color: #94a3b8;
  font-size: 12px;
}
</style>
