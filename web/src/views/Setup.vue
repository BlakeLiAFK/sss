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
        <el-step :title="t('setup.stepWelcome')" />
        <el-step :title="t('setup.stepConfig')" />
        <el-step :title="t('setup.stepComplete')" />
      </el-steps>

      <!-- 步骤 1: 欢迎 -->
      <div v-if="currentStep === 0" class="step-content">
        <div class="welcome-content">
          <el-icon :size="64" color="#e67e22"><Setting /></el-icon>
          <h2>{{ t('setup.welcomeTitle') }}</h2>
          <p>{{ t('setup.welcomeDesc') }}</p>
          <p>{{ t('setup.welcomeHint') }}</p>
          <ul>
            <li>{{ t('setup.setupAdminAccount') }}</li>
            <li>{{ t('setup.configServer') }}</li>
          </ul>
        </div>
        <el-button type="primary" size="large" @click="currentStep = 1" class="next-button">
          {{ t('setup.startConfig') }}
        </el-button>
        <LanguageSwitcher size="small" :show-label="false" class="lang-switcher" />
      </div>

      <!-- 步骤 2: 配置 -->
      <div v-if="currentStep === 1" class="step-content">
        <el-form :model="form" :rules="rules" ref="formRef" label-position="top">
          <el-divider :content-position="'left'">{{ t('setup.adminAccount') }}</el-divider>

          <el-form-item :label="t('setup.adminUsername')" prop="adminUsername">
            <el-input v-model="form.adminUsername" :placeholder="t('setup.adminUsernamePlaceholder')" :prefix-icon="User" />
          </el-form-item>

          <el-form-item :label="t('setup.adminPassword')" prop="adminPassword">
            <el-input
              v-model="form.adminPassword"
              type="password"
              :placeholder="t('setup.atLeast6Chars')"
              :prefix-icon="Lock"
              show-password
            />
          </el-form-item>

          <el-form-item :label="t('setup.confirmPassword')" prop="confirmPassword">
            <el-input
              v-model="form.confirmPassword"
              type="password"
              :placeholder="t('setup.enterAgain')"
              :prefix-icon="Lock"
              show-password
            />
          </el-form-item>

          <el-divider :content-position="'left'">{{ t('setup.serverConfig') }}</el-divider>

          <el-row :gutter="16">
            <el-col :span="16">
              <el-form-item :label="t('setup.listenAddress')">
                <el-input v-model="form.serverHost" :placeholder="t('setup.listenAddressPlaceholder')" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item :label="t('setup.port')">
                <el-input v-model="form.serverPort" :placeholder="t('setup.portPlaceholder')" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item :label="t('setup.region')">
            <el-input v-model="form.serverRegion" :placeholder="t('setup.regionPlaceholder')" />
          </el-form-item>
        </el-form>

        <div class="button-group">
          <el-button size="large" @click="currentStep = 0">{{ t('common.previous') }}</el-button>
          <el-button type="primary" size="large" @click="handleInstall" :loading="loading">
            {{ t('setup.install') }}
          </el-button>
        </div>
      </div>

      <!-- 步骤 3: 完成 -->
      <div v-if="currentStep === 2" class="step-content">
        <div class="success-content">
          <el-icon :size="64" color="#10b981"><CircleCheck /></el-icon>
          <h2>{{ t('setup.completeTitle') }}</h2>
          <p>{{ t('setup.completeDesc') }}</p>
          <p class="hint-text">{{ t('setup.completeHint') }}</p>
        </div>

        <el-button type="primary" size="large" @click="goToLogin" class="next-button">
          {{ t('setup.goToLogin') }}
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, type FormInstance } from 'element-plus'
import { User, Lock, Setting, CircleCheck } from '@element-plus/icons-vue'
import LanguageSwitcher from '../components/LanguageSwitcher.vue'
import { resetInstallStatus } from '../router'
import apiClient from '../api/client'

const { t } = useI18n()

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


// 密码确认验证
const validateConfirmPassword = (_rule: any, value: string, callback: Function) => {
  if (value !== form.adminPassword) {
    callback(new Error(t('setup.passwordMismatch')))
  } else {
    callback()
  }
}

const rules = {
  adminUsername: [
    { required: true, message: t('setup.pleaseEnterUsername'), trigger: 'blur' },
    { min: 3, message: t('setup.atLeast3Chars'), trigger: 'blur' }
  ],
  adminPassword: [
    { required: true, message: t('setup.pleaseEnterPassword'), trigger: 'blur' },
    { min: 6, message: t('setup.atLeast6Chars'), trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: t('setup.pleaseConfirmPassword'), trigger: 'blur' },
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
        currentStep.value = 2
        ElMessage.success(t('setup.installSuccess'))
      } else {
        ElMessage.error(response.data.message || t('setup.installFailed'))
      }
    } catch (e: any) {
      ElMessage.error(t('setup.installFailed') + ': ' + (e.response?.data?.Message || e.message))
    } finally {
      loading.value = false
    }
  })
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
  background: linear-gradient(135deg, #e67e22 0%, #d35400 100%);
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

.hint-text {
  font-size: 13px;
  color: #94a3b8;
  margin-top: 16px !important;
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
