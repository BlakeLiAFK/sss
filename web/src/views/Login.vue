<template>
  <div class="login-container">
    <div class="login-box">
      <div class="login-header">
        <div class="logo-wrapper">
          <el-icon :size="40"><Box /></el-icon>
        </div>
        <h1 class="title">{{ t('login.title') }}</h1>
        <p class="subtitle">{{ t('login.subtitle') }}</p>
        <LanguageSwitcher size="small" :show-label="false" class="lang-switcher" />
      </div>

      <el-form :model="form" :rules="rules" ref="formRef" class="login-form">
        <el-form-item prop="username">
          <el-input
            v-model="form.username"
            :placeholder="t('login.username')"
            :prefix-icon="User"
            size="large"
          />
        </el-form-item>
        <el-form-item prop="password">
          <el-input
            v-model="form.password"
            type="password"
            :placeholder="t('login.password')"
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
            {{ t('login.login') }}
          </el-button>
        </el-form-item>
      </el-form>

      <div class="login-footer">
        <el-button link type="primary" @click="openResetDialog">
          {{ t('login.forgotPassword') }}
        </el-button>
      </div>
    </div>

    <!-- 密码重置对话框 -->
    <el-dialog v-model="showResetDialog" :title="t('login.resetPasswordTitle')" width="min(450px, 95vw)" :close-on-click-modal="!resetting" :close-on-press-escape="!resetting" :show-close="!resetting">
      <div v-if="!resetFileExists">
        <el-alert type="info" :closable="false" show-icon>
          <template #title>
            <span>{{ t('login.resetCommand') }}</span>
          </template>
          <template #default>{{ t('login.resetCommandHint') }}</template>
        </el-alert>

        <el-button type="primary" @click="checkResetFile(true)" :loading="checkingFile">
          {{ t('login.checkFile') }}
        </el-button>
      </div>

      <div v-else>
        <el-form :model="resetForm" :rules="resetRules" ref="resetFormRef">
          <el-form-item :label="t('login.newPassword')" prop="newPassword">
            <el-input
              v-model="resetForm.newPassword"
              type="password"
              :placeholder="t('login.passwordRequirements')"
              show-password
            />
          </el-form-item>
          <el-form-item :label="t('login.confirmPassword')" prop="confirmPassword">
            <el-input
              v-model="resetForm.confirmPassword"
              type="password"
              :placeholder="t('login.enterAgain')"
              show-password
            />
          </el-form-item>
        </el-form>
        <el-button type="primary" @click="handleResetPassword" :loading="resetting">
          {{ t('login.resetPassword') }}
        </el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, type FormInstance } from 'element-plus'
import { useAuthStore } from '../stores/auth'
import { User, Lock } from '@element-plus/icons-vue'
import LanguageSwitcher from '../components/LanguageSwitcher.vue'
import axios from 'axios'
import { getBaseUrl } from '../api/client'

const { t } = useI18n()

const router = useRouter()
const auth = useAuthStore()
const formRef = ref<FormInstance>()
const resetFormRef = ref<FormInstance>()
const loading = ref(false)

// 服务器地址始终使用当前域名
const endpoint = getBaseUrl()

const form = reactive({
  username: 'admin',
  password: ''
})

const rules = {
  username: [{ required: true, message: t('login.pleaseEnterUsername'), trigger: 'blur' }],
  password: [{ required: true, message: t('login.pleaseEnterPassword'), trigger: 'blur' }]
}

// 密码重置相关
const showResetDialog = ref(false)
const resetFileExists = ref(false)
const checkingFile = ref(false)
const resetting = ref(false)

const resetForm = reactive({
  newPassword: '',
  confirmPassword: ''
})

const validateConfirmPassword = (_rule: any, value: string, callback: Function) => {
  if (value !== resetForm.newPassword) {
    callback(new Error(t('login.passwordMismatch')))
  } else {
    callback()
  }
}

const resetRules = {
  newPassword: [
    { required: true, message: t('login.pleaseEnterPassword'), trigger: 'blur' },
    { pattern: /^(?=.*[a-z])(?=.*[A-Z])(?=.*[0-9]).{8,}$/s, message: t('login.passwordRequirements'), trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: t('login.pleaseConfirmPassword'), trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' }
  ]
}

async function handleLogin() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return

    loading.value = true
    try {
      const response = await axios.post(`${endpoint}/api/admin/login`, {
        username: form.username,
        password: form.password
      })

      if (response.data.success) {
        auth.login(
          response.data.token,
          endpoint,
          response.data.accessKeyId || '',
          response.data.secretAccessKey || ''
        )
        ElMessage.success(t('login.loginSuccess'))
        router.push('/')
      } else {
        ElMessage.error(response.data.message || response.data.Message || t('login.loginFailed'))
      }
    } catch (e: any) {
      const msg = e.response?.data?.Message || e.response?.data?.message || e.message
      ElMessage.error(t('login.loginFailed') + ': ' + msg)
    } finally {
      loading.value = false
    }
  })
}

// 检测重置文件
async function openResetDialog() {
  resetFileExists.value = false
  resetForm.newPassword = ''
  resetForm.confirmPassword = ''
  showResetDialog.value = true
  await checkResetFile(false)
}

async function checkResetFile(notify = true) {
  checkingFile.value = true
  try {
    const response = await axios.get(`${endpoint}/api/setup/reset-password/check`)
    resetFileExists.value = response.data.file_exists === true
    if (resetFileExists.value) {
      if (notify) ElMessage.success(t('login.fileDetected'))
    } else {
      if (notify) ElMessage.warning(t('login.fileNotDetected'))
    }
  } catch (e: any) {
    resetFileExists.value = false
    ElMessage.error(t('login.checkFailed') + ': ' + (e.response?.data?.Message || e.response?.data?.message || e.message))
  } finally {
    checkingFile.value = false
  }
}

// 重置密码
async function handleResetPassword() {
  if (!resetFormRef.value) return

  await resetFormRef.value.validate(async (valid) => {
    if (!valid) return

    resetting.value = true
    try {
      const response = await axios.post(`${endpoint}/api/setup/reset-password`, {
        new_password: resetForm.newPassword
      })

      if (response.data.success) {
        ElMessage.success(t('login.resetSuccess'))
        showResetDialog.value = false
        resetFileExists.value = false
        resetForm.newPassword = ''
        resetForm.confirmPassword = ''
      } else {
        ElMessage.error(response.data.message || t('login.resetFailed'))
      }
    } catch (e: any) {
      if (e.response?.status === 403) resetFileExists.value = false
      ElMessage.error(t('login.resetFailed') + ': ' + (e.response?.data?.Message || e.response?.data?.message || e.message))
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
  box-shadow: 0 0 0 2px #e67e22;
}

.login-button {
  width: 100%;
  height: 44px;
  font-size: 16px;
  font-weight: 600;
  border-radius: 8px;
  background: linear-gradient(135deg, #e67e22 0%, #d35400 100%);
  border: none;
}

.login-button:hover {
  background: linear-gradient(135deg, #d35400 0%, #c0392b 100%);
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

.lang-switcher {
  position: absolute;
  top: 20px;
  right: 20px;
}

:deep(.lang-switcher .el-button) {
  background: rgba(255, 255, 255, 0.2);
  border-color: rgba(255, 255, 255, 0.3);
  color: #ffffff;
}

:deep(.lang-switcher .el-button:hover) {
  background: rgba(255, 255, 255, 0.3);
  border-color: rgba(255, 255, 255, 0.5);
}
</style>
