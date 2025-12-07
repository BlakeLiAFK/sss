<template>
  <div class="setup-container">
    <div class="setup-box">
      <div class="setup-header">
        <div class="logo-wrapper">
          <el-icon :size="40"><Box /></el-icon>
        </div>
        <h1 class="title">SSS</h1>
        <p class="subtitle">系统初始化</p>
      </div>

      <!-- 步骤指示器 -->
      <el-steps :active="currentStep" finish-status="success" simple class="setup-steps">
        <el-step title="欢迎" />
        <el-step title="配置" />
        <el-step title="完成" />
      </el-steps>

      <!-- 步骤 1: 欢迎 -->
      <div v-if="currentStep === 0" class="step-content">
        <div class="welcome-content">
          <el-icon :size="64" color="#3b82f6"><Setting /></el-icon>
          <h2>欢迎使用 SSS</h2>
          <p>Simple S3 Server 是一个轻量级、自托管的 S3 兼容对象存储服务器。</p>
          <p>首次使用需要完成以下初始化配置：</p>
          <ul>
            <li>设置管理员账号和密码</li>
            <li>配置服务器参数</li>
            <li>生成 API 访问密钥</li>
          </ul>
        </div>
        <el-button type="primary" size="large" @click="currentStep = 1" class="next-button">
          开始配置
        </el-button>
      </div>

      <!-- 步骤 2: 配置 -->
      <div v-if="currentStep === 1" class="step-content">
        <el-form :model="form" :rules="rules" ref="formRef" label-position="top">
          <el-divider content-position="left">管理员账号</el-divider>
          
          <el-form-item label="用户名" prop="adminUsername">
            <el-input v-model="form.adminUsername" placeholder="admin" :prefix-icon="User" />
          </el-form-item>
          
          <el-form-item label="密码" prop="adminPassword">
            <el-input 
              v-model="form.adminPassword" 
              type="password" 
              placeholder="至少6位字符" 
              :prefix-icon="Lock"
              show-password 
            />
          </el-form-item>
          
          <el-form-item label="确认密码" prop="confirmPassword">
            <el-input 
              v-model="form.confirmPassword" 
              type="password" 
              placeholder="再次输入密码" 
              :prefix-icon="Lock"
              show-password 
            />
          </el-form-item>

          <el-divider content-position="left">服务器配置（可选）</el-divider>
          
          <el-row :gutter="16">
            <el-col :span="16">
              <el-form-item label="监听地址">
                <el-input v-model="form.serverHost" placeholder="0.0.0.0" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="端口">
                <el-input v-model="form.serverPort" placeholder="8080" />
              </el-form-item>
            </el-col>
          </el-row>
          
          <el-form-item label="区域">
            <el-input v-model="form.serverRegion" placeholder="us-east-1" />
          </el-form-item>
        </el-form>

        <div class="button-group">
          <el-button size="large" @click="currentStep = 0">上一步</el-button>
          <el-button type="primary" size="large" @click="handleInstall" :loading="loading">
            完成安装
          </el-button>
        </div>
      </div>

      <!-- 步骤 3: 完成 -->
      <div v-if="currentStep === 2" class="step-content">
        <div class="success-content">
          <el-icon :size="64" color="#10b981"><CircleCheck /></el-icon>
          <h2>安装成功！</h2>
          <p>请保存以下 API 访问密钥，它只会显示一次：</p>
          
          <div class="key-display">
            <div class="key-item">
              <span class="key-label">Access Key ID:</span>
              <code class="key-value">{{ result.accessKeyId }}</code>
              <el-button text @click="copyToClipboard(result.accessKeyId)">
                <el-icon><DocumentCopy /></el-icon>
              </el-button>
            </div>
            <div class="key-item">
              <span class="key-label">Secret Access Key:</span>
              <code class="key-value">{{ result.secretAccessKey }}</code>
              <el-button text @click="copyToClipboard(result.secretAccessKey)">
                <el-icon><DocumentCopy /></el-icon>
              </el-button>
            </div>
          </div>

          <el-alert 
            title="重要提示" 
            type="warning" 
            description="Secret Access Key 只会显示一次，请妥善保存。如果丢失，需要在管理后台重新生成。"
            show-icon
            :closable="false"
            class="warning-alert"
          />
        </div>
        
        <el-button type="primary" size="large" @click="goToLogin" class="next-button">
          前往登录
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance } from 'element-plus'
import { User, Lock, Setting, CircleCheck, DocumentCopy } from '@element-plus/icons-vue'
import { resetInstallStatus } from '../router'
import apiClient from '../api/client'

const router = useRouter()
const formRef = ref<FormInstance>()
const loading = ref(false)
const currentStep = ref(0)

const form = reactive({
  adminUsername: 'admin',
  adminPassword: '',
  confirmPassword: '',
  serverHost: '0.0.0.0',
  serverPort: '8080',
  serverRegion: 'us-east-1'
})

const result = reactive({
  accessKeyId: '',
  secretAccessKey: ''
})

// 密码确认验证
const validateConfirmPassword = (_rule: any, value: string, callback: Function) => {
  if (value !== form.adminPassword) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const rules = {
  adminUsername: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, message: '用户名至少3个字符', trigger: 'blur' }
  ],
  adminPassword: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码至少6位字符', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' }
  ]
}

async function handleInstall() {
  if (!formRef.value) return
  
  await formRef.value.validate(async (valid) => {
    if (!valid) return

    loading.value = true
    try {
      const response = await apiClient.post('/api/setup/install', {
        admin_username: form.adminUsername,
        admin_password: form.adminPassword,
        server_host: form.serverHost,
        server_port: form.serverPort,
        server_region: form.serverRegion
      })

      if (response.data.success) {
        result.accessKeyId = response.data.access_key_id
        result.secretAccessKey = response.data.secret_access_key
        currentStep.value = 2
        ElMessage.success('安装成功')
      } else {
        ElMessage.error(response.data.message || '安装失败')
      }
    } catch (e: any) {
      ElMessage.error('安装失败: ' + (e.response?.data?.Message || e.message))
    } finally {
      loading.value = false
    }
  })
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text)
  ElMessage.success('已复制到剪贴板')
}

async function goToLogin() {
  try {
    resetInstallStatus() // 重置安装状态缓存
    await router.push({ name: 'Login' })
  } catch (e) {
    console.error('跳转失败:', e)
    // 强制跳转
    window.location.href = '/#/login'
  }
}
</script>

<style scoped>
.setup-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
  padding: 20px;
}

.setup-box {
  width: 100%;
  max-width: 500px;
  background: #ffffff;
  border-radius: 12px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  padding: 40px;
}

.setup-header {
  text-align: center;
  margin-bottom: 24px;
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

.setup-steps {
  margin-bottom: 32px;
}

.step-content {
  min-height: 300px;
}

.welcome-content {
  text-align: center;
  padding: 20px 0;
}

.welcome-content h2 {
  margin: 16px 0 8px;
  color: #1e293b;
}

.welcome-content p {
  color: #64748b;
  margin: 8px 0;
}

.welcome-content ul {
  text-align: left;
  color: #475569;
  padding-left: 20px;
  margin: 16px auto;
  max-width: 280px;
}

.welcome-content li {
  margin: 8px 0;
}

.success-content {
  text-align: center;
  padding: 20px 0;
}

.success-content h2 {
  margin: 16px 0 8px;
  color: #1e293b;
}

.success-content p {
  color: #64748b;
  margin: 8px 0;
}

.key-display {
  background: #f8fafc;
  border-radius: 8px;
  padding: 16px;
  margin: 20px 0;
  text-align: left;
}

.key-item {
  display: flex;
  align-items: center;
  margin: 8px 0;
  flex-wrap: wrap;
}

.key-label {
  font-weight: 600;
  color: #475569;
  min-width: 140px;
}

.key-value {
  font-family: monospace;
  background: #e2e8f0;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 13px;
  word-break: break-all;
  flex: 1;
}

.warning-alert {
  margin-top: 16px;
  text-align: left;
}

.next-button {
  width: 100%;
  height: 44px;
  font-size: 16px;
  font-weight: 600;
  border-radius: 8px;
  margin-top: 20px;
}

.button-group {
  display: flex;
  gap: 12px;
  margin-top: 20px;
}

.button-group .el-button {
  flex: 1;
  height: 44px;
}

:deep(.el-divider__text) {
  color: #64748b;
  font-size: 13px;
}
</style>
