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
            登录
          </el-button>
        </el-form-item>
      </el-form>

      <div class="login-footer">
        <el-button link type="primary" @click="showResetDialog = true">
          忘记密码？
        </el-button>
      </div>
    </div>

    <!-- 密码重置对话框 -->
    <el-dialog v-model="showResetDialog" title="重置管理员密码" width="450px">
      <div v-if="!resetFileExists">
        <el-alert type="info" :closable="false" show-icon>
          <template #title>
            <span>请在服务器上执行以下命令：</span>
          </template>
          <template #default>
            <code class="reset-command">{{ resetCommand }}</code>
            <el-button size="small" @click="copyResetCommand" style="margin-left: 8px;">
              复制
            </el-button>
          </template>
        </el-alert>
        <p class="reset-tip">执行完成后，点击下方按钮检测文件。</p>
        <el-button type="primary" @click="checkResetFile" :loading="checkingFile">
          检测文件
        </el-button>
      </div>

      <div v-else>
        <el-form :model="resetForm" :rules="resetRules" ref="resetFormRef">
          <el-form-item label="新密码" prop="newPassword">
            <el-input 
              v-model="resetForm.newPassword" 
              type="password" 
              placeholder="至少6位字符"
              show-password
            />
          </el-form-item>
          <el-form-item label="确认密码" prop="confirmPassword">
            <el-input 
              v-model="resetForm.confirmPassword" 
              type="password" 
              placeholder="再次输入密码"
              show-password
            />
          </el-form-item>
        </el-form>
        <el-button type="primary" @click="handleResetPassword" :loading="resetting">
          重置密码
        </el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance } from 'element-plus'
import { useAuthStore } from '../stores/auth'
import { User, Lock, Link, Location } from '@element-plus/icons-vue'
import axios from 'axios'
import { getBaseUrl } from '../api/client'

const router = useRouter()
const auth = useAuthStore()
const formRef = ref<FormInstance>()
const resetFormRef = ref<FormInstance>()
const loading = ref(false)

const form = reactive({
  endpoint: auth.endpoint || getBaseUrl(),
  region: auth.region || 'us-east-1',
  username: 'admin',
  password: ''
})

const rules = {
  endpoint: [{ required: true, message: '请输入服务器地址', trigger: 'blur' }],
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

// 密码重置相关
const showResetDialog = ref(false)
const resetFileExists = ref(false)
const resetCommand = ref('')
const checkingFile = ref(false)
const resetting = ref(false)

const resetForm = reactive({
  newPassword: '',
  confirmPassword: ''
})

const validateConfirmPassword = (_rule: any, value: string, callback: Function) => {
  if (value !== resetForm.newPassword) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const resetRules = {
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码至少6位字符', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' }
  ]
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
        auth.login(
          response.data.token,
          form.endpoint,
          form.region,
          response.data.accessKeyId || '',
          response.data.secretAccessKey || ''
        )
        ElMessage.success('登录成功')
        router.push('/')
      } else {
        ElMessage.error(response.data.message || response.data.Message || '登录失败')
      }
    } catch (e: any) {
      const msg = e.response?.data?.Message || e.response?.data?.message || e.message
      ElMessage.error('登录失败: ' + msg)
    } finally {
      loading.value = false
    }
  })
}

// 检测重置文件
async function checkResetFile() {
  checkingFile.value = true
  try {
    const response = await axios.get(`${form.endpoint}/api/setup/reset-password/check`)
    if (response.data.file_exists) {
      resetFileExists.value = true
      ElMessage.success('检测到重置文件，请设置新密码')
    } else {
      resetCommand.value = response.data.command
      ElMessage.warning('未检测到重置文件，请先在服务器执行命令')
    }
  } catch (e: any) {
    ElMessage.error('检测失败: ' + (e.response?.data?.Message || e.message))
  } finally {
    checkingFile.value = false
  }
}

// 复制重置命令
function copyResetCommand() {
  navigator.clipboard.writeText(resetCommand.value)
  ElMessage.success('已复制到剪贴板')
}

// 重置密码
async function handleResetPassword() {
  if (!resetFormRef.value) return
  
  await resetFormRef.value.validate(async (valid) => {
    if (!valid) return

    resetting.value = true
    try {
      const response = await axios.post(`${form.endpoint}/api/setup/reset-password`, {
        new_password: resetForm.newPassword
      })
      
      if (response.data.success) {
        ElMessage.success('密码重置成功，请使用新密码登录')
        showResetDialog.value = false
        resetFileExists.value = false
        resetForm.newPassword = ''
        resetForm.confirmPassword = ''
      } else {
        ElMessage.error(response.data.message || '重置失败')
      }
    } catch (e: any) {
      ElMessage.error('重置失败: ' + (e.response?.data?.Message || e.message))
    } finally {
      resetting.value = false
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
  margin-top: 16px;
}

.reset-command {
  display: block;
  background: #f1f5f9;
  padding: 8px 12px;
  border-radius: 4px;
  font-family: monospace;
  margin: 8px 0;
  word-break: break-all;
}

.reset-tip {
  color: #64748b;
  font-size: 14px;
  margin: 16px 0;
}
</style>
