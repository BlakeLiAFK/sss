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
        <el-form-item prop="endpoint">
          <el-input
            v-model="form.endpoint"
            :placeholder="t('login.serverEndpoint')"
            :prefix-icon="Link"
            size="large"
          />
        </el-form-item>
        <el-form-item prop="region">
          <el-input
            v-model="form.region"
            :placeholder="t('login.region')"
            :prefix-icon="Location"
            size="large"
          />
        </el-form-item>
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
        <el-button link type="primary" @click="showResetDialog = true">
          {{ t('login.forgotPassword') }}
        </el-button>
      </div>
    </div>

    <!-- 密码重置对话框 -->
    <el-dialog v-model="showResetDialog" :title="t('login.resetPasswordTitle')" width="450px">
      <div v-if="!resetFileExists">
        <el-alert type="info" :closable="false" show-icon>
          <template #title>
            <span>{{ t('login.resetCommand') }}</span>
          </template>
          <template #default>
            <code class="reset-command">{{ resetCommand }}</code>
            <el-button size="small" @click="copyResetCommand" style="margin-left: 8px;">
              {{ t('common.copy') }}
            </el-button>
          </template>
        </el-alert>
        <p class="reset-tip">{{ t('login.resetCommandHint') }}</p>
        <el-button type="primary" @click="checkResetFile" :loading="checkingFile">
          {{ t('login.checkFile') }}
        </el-button>
      </div>

      <div v-else>
        <el-form :model="resetForm" :rules="resetRules" ref="resetFormRef">
          <el-form-item :label="t('login.newPassword')" prop="newPassword">
            <el-input
              v-model="resetForm.newPassword"
              type="password"
              :placeholder="t('login.atLeast6Chars')"
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
import { User, Lock, Link, Location } from '@element-plus/icons-vue'
import LanguageSwitcher from '../components/LanguageSwitcher.vue'
import axios from 'axios'
import { getBaseUrl } from '../api/client'

const { t } = useI18n()

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
  endpoint: [{ required: true, message: t('login.pleaseEnterEndpoint'), trigger: 'blur' }],
  username: [{ required: true, message: t('login.pleaseEnterUsername'), trigger: 'blur' }],
  password: [{ required: true, message: t('login.pleaseEnterPassword'), trigger: 'blur' }]
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
    callback(new Error(t('login.passwordMismatch')))
  } else {
    callback()
  }
}

const resetRules = {
  newPassword: [
    { required: true, message: t('login.pleaseEnterPassword'), trigger: 'blur' },
    { min: 6, message: t('login.atLeast6Chars'), trigger: 'blur' }
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
async function checkResetFile() {
  checkingFile.value = true
  try {
    const response = await axios.get(`${form.endpoint}/api/setup/reset-password/check`)
    if (response.data.file_exists) {
      resetFileExists.value = true
      ElMessage.success(t('login.fileDetected'))
    } else {
      resetCommand.value = response.data.command
      ElMessage.warning(t('login.fileNotDetected'))
    }
  } catch (e: any) {
    ElMessage.error(t('login.checkFailed') + ': ' + (e.response?.data?.Message || e.message))
  } finally {
    checkingFile.value = false
  }
}

// 复制重置命令（兼容 HTTP 环境）
function copyResetCommand() {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(resetCommand.value)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = resetCommand.value
      textarea.style.position = 'fixed'
      textarea.style.left = '-9999px'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }
    ElMessage.success(t('common.copied'))
  } catch {
    ElMessage.error(t('common.copyFailed'))
  }
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
        ElMessage.success(t('login.resetSuccess'))
        showResetDialog.value = false
        resetFileExists.value = false
        resetForm.newPassword = ''
        resetForm.confirmPassword = ''
      } else {
        ElMessage.error(response.data.message || t('login.resetFailed'))
      }
    } catch (e: any) {
      ElMessage.error(t('login.resetFailed') + ': ' + (e.response?.data?.Message || e.message))
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
