<template>
  <div class="login-container">
    <el-card class="login-card">
      <template #header>
        <div class="card-header">
          <el-icon :size="32" color="#409EFF"><Box /></el-icon>
          <span>SSS - Simple S3 Server</span>
        </div>
      </template>

      <el-form :model="form" :rules="rules" ref="formRef" label-width="120px">
        <el-form-item label="Endpoint" prop="endpoint">
          <el-input v-model="form.endpoint" placeholder="http://localhost:8080" />
        </el-form-item>
        <el-form-item label="Region" prop="region">
          <el-input v-model="form.region" placeholder="us-east-1" />
        </el-form-item>
        <el-form-item label="Access Key ID" prop="accessKeyId">
          <el-input v-model="form.accessKeyId" placeholder="admin" />
        </el-form-item>
        <el-form-item label="Secret Key" prop="secretAccessKey">
          <el-input v-model="form.secretAccessKey" type="password" placeholder="admin" show-password />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleLogin" :loading="loading" style="width: 100%">
            登录
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
import { listBuckets } from '../api/s3'

const router = useRouter()
const auth = useAuthStore()
const formRef = ref<FormInstance>()
const loading = ref(false)

const form = reactive({
  endpoint: auth.endpoint || 'http://localhost:8080',
  region: auth.region || 'us-east-1',
  accessKeyId: auth.accessKeyId || 'admin',
  secretAccessKey: auth.secretAccessKey || 'admin'
})

const rules = {
  endpoint: [{ required: true, message: '请输入Endpoint', trigger: 'blur' }],
  accessKeyId: [{ required: true, message: '请输入Access Key ID', trigger: 'blur' }],
  secretAccessKey: [{ required: true, message: '请输入Secret Access Key', trigger: 'blur' }]
}

async function handleLogin() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return

    loading.value = true
    try {
      auth.login(form.accessKeyId, form.secretAccessKey, form.endpoint, form.region)
      await listBuckets() // 测试连接
      ElMessage.success('登录成功')
      router.push('/')
    } catch (e: any) {
      auth.logout()
      ElMessage.error('登录失败: ' + (e.response?.data || e.message))
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
  width: 450px;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 20px;
  font-weight: 600;
}
</style>
