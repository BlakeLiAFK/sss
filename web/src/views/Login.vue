<template>
  <div class="login-container">
    <el-card class="login-card">
      <template #header>
        <div class="card-header">
          <el-icon :size="32" color="#409EFF"><Box /></el-icon>
          <span>SSS - Simple S3 Server</span>
        </div>
      </template>

      <el-form :model="form" :rules="rules" ref="formRef" label-width="100px">
        <el-form-item label="Endpoint" prop="endpoint">
          <el-input v-model="form.endpoint" placeholder="http://localhost:8080" />
        </el-form-item>
        <el-form-item label="Region" prop="region">
          <el-input v-model="form.region" placeholder="us-east-1" />
        </el-form-item>
        <el-form-item label="Username" prop="username">
          <el-input v-model="form.username" placeholder="admin" />
        </el-form-item>
        <el-form-item label="Password" prop="password">
          <el-input v-model="form.password" type="password" placeholder="admin" show-password />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleLogin" :loading="loading" style="width: 100%">
            Login
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance } from 'element-plus'
import { useAuthStore } from '../stores/auth'
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
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.login-card {
  width: 420px;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 20px;
  font-weight: 600;
}
</style>
