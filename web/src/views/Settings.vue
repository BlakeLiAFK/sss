<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title">
        <h1>系统设置</h1>
        <p class="page-subtitle">管理存储配置与安全选项</p>
      </div>
    </div>

    <div class="settings-grid">
      <!-- 运行时参数（只读） -->
      <div class="settings-card">
        <div class="card-header">
          <el-icon class="card-icon"><Monitor /></el-icon>
          <h3>运行时参数</h3>
          <el-tag size="small" type="info">只读</el-tag>
        </div>
        <div class="card-body">
          <div class="info-item">
            <span class="info-label">监听地址</span>
            <span class="info-value mono">{{ settings.runtime.host }}:{{ settings.runtime.port }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">数据目录</span>
            <span class="info-value mono">{{ settings.runtime.data_path }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">数据库</span>
            <span class="info-value mono">{{ settings.runtime.db_path }}</span>
          </div>
          <p class="setting-hint">这些参数通过命令行设置，需重启服务才能修改</p>
        </div>
      </div>

      <!-- 存储配置（可修改） -->
      <div class="settings-card">
        <div class="card-header">
          <el-icon class="card-icon"><FolderOpened /></el-icon>
          <h3>存储配置</h3>
        </div>
        <div class="card-body">
          <div class="setting-item">
            <label>S3 区域</label>
            <el-input v-model="settings.storage.region" placeholder="us-east-1" :disabled="!editing" />
          </div>
          <div class="setting-item">
            <label>最大对象大小</label>
            <el-select v-model="maxObjectSizeOption" :disabled="!editing" style="width: 100%">
              <el-option label="1 GB" :value="1073741824" />
              <el-option label="2 GB" :value="2147483648" />
              <el-option label="5 GB" :value="5368709120" />
              <el-option label="10 GB" :value="10737418240" />
            </el-select>
            <span class="setting-hint">单个对象允许的最大大小</span>
          </div>
          <div class="setting-item">
            <label>预签名上传限制</label>
            <el-select v-model="maxUploadSizeOption" :disabled="!editing" style="width: 100%">
              <el-option label="100 MB" :value="104857600" />
              <el-option label="500 MB" :value="524288000" />
              <el-option label="1 GB" :value="1073741824" />
              <el-option label="2 GB" :value="2147483648" />
            </el-select>
            <span class="setting-hint">预签名 URL 上传的最大大小</span>
          </div>
        </div>
      </div>

      <!-- 系统信息 -->
      <div class="settings-card">
        <div class="card-header">
          <el-icon class="card-icon"><InfoFilled /></el-icon>
          <h3>系统信息</h3>
        </div>
        <div class="card-body">
          <div class="info-item">
            <span class="info-label">版本</span>
            <span class="info-value">{{ settings.system.version || '-' }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">安装时间</span>
            <span class="info-value">{{ formatDate(settings.system.installed_at) }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">状态</span>
            <el-tag type="success" size="small">运行中</el-tag>
          </div>
        </div>
      </div>

      <!-- 安全设置 -->
      <div class="settings-card">
        <div class="card-header">
          <el-icon class="card-icon"><Lock /></el-icon>
          <h3>安全设置</h3>
        </div>
        <div class="card-body">
          <div class="setting-item">
            <label>CORS 允许来源</label>
            <el-input v-model="settings.security.cors_origin" placeholder="*" :disabled="!editing" />
            <span class="setting-hint">允许跨域请求的来源，* 表示允许所有来源</span>
          </div>
          <div class="setting-item">
            <label>预签名 URL 协议</label>
            <el-select v-model="settings.security.presign_scheme" :disabled="!editing" style="width: 100%">
              <el-option label="HTTP" value="http" />
              <el-option label="HTTPS" value="https" />
            </el-select>
            <span class="setting-hint">生成预签名 URL 时使用的协议</span>
          </div>
          <div class="setting-item" style="margin-top: 16px;">
            <el-button type="primary" @click="showPasswordDialog = true" class="full-width-btn">
              <el-icon><Key /></el-icon>
              修改管理员密码
            </el-button>
          </div>
        </div>
      </div>
    </div>

    <!-- 操作按钮 -->
    <div class="action-bar">
      <template v-if="!editing">
        <el-button type="primary" @click="editing = true" class="primary-btn">
          <el-icon><Edit /></el-icon>
          编辑设置
        </el-button>
      </template>
      <template v-else>
        <el-button @click="cancelEdit">取消</el-button>
        <el-button type="primary" @click="saveSettings" :loading="saving" class="primary-btn">
          <el-icon><Check /></el-icon>
          保存设置
        </el-button>
      </template>
    </div>

    <!-- 修改密码对话框 -->
    <el-dialog v-model="showPasswordDialog" title="修改管理员密码" width="400px" :close-on-click-modal="false">
      <el-form :model="passwordForm" label-position="top">
        <el-form-item label="当前密码" required>
          <el-input v-model="passwordForm.oldPassword" type="password" show-password placeholder="输入当前密码" />
        </el-form-item>
        <el-form-item label="新密码" required>
          <el-input v-model="passwordForm.newPassword" type="password" show-password placeholder="至少6个字符" />
        </el-form-item>
        <el-form-item label="确认新密码" required>
          <el-input v-model="passwordForm.confirmPassword" type="password" show-password placeholder="再次输入新密码" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showPasswordDialog = false">取消</el-button>
        <el-button type="primary" @click="changePassword" :loading="changingPassword" class="primary-btn">
          确认修改
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Monitor, FolderOpened, InfoFilled, Lock, Key, Edit, Check } from '@element-plus/icons-vue'
import { useAuthStore } from '../stores/auth'
import axios from 'axios'

const auth = useAuthStore()

// 状态
const loading = ref(false)
const editing = ref(false)
const saving = ref(false)
const showPasswordDialog = ref(false)
const changingPassword = ref(false)

// 设置数据（匹配新 API 结构）
const settings = reactive({
  runtime: {
    host: '',
    port: 8080,
    data_path: '',
    db_path: ''
  },
  storage: {
    region: '',
    max_object_size: 0,
    max_upload_size: 0
  },
  security: {
    cors_origin: '*',
    presign_scheme: 'http'
  },
  system: {
    installed: false,
    installed_at: '',
    version: ''
  }
})

// 原始设置备份
const originalSettings = ref<typeof settings | null>(null)

// 大小选项
const maxObjectSizeOption = computed({
  get: () => settings.storage.max_object_size,
  set: (val) => { settings.storage.max_object_size = val }
})

const maxUploadSizeOption = computed({
  get: () => settings.storage.max_upload_size,
  set: (val) => { settings.storage.max_upload_size = val }
})

// 密码表单
const passwordForm = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: ''
})

function getHeaders() {
  return {
    'X-Admin-Token': auth.adminToken,
    'Content-Type': 'application/json'
  }
}

async function loadSettings() {
  loading.value = true
  try {
    const response = await axios.get(`${auth.endpoint}/api/admin/settings`, {
      headers: getHeaders()
    })
    Object.assign(settings.runtime, response.data.runtime)
    Object.assign(settings.storage, response.data.storage)
    if (response.data.security) {
      Object.assign(settings.security, response.data.security)
    }
    Object.assign(settings.system, response.data.system)
    originalSettings.value = JSON.parse(JSON.stringify(settings))
  } catch (error: any) {
    ElMessage.error('加载设置失败: ' + (error.response?.data?.message || error.message))
  } finally {
    loading.value = false
  }
}

function cancelEdit() {
  if (originalSettings.value) {
    Object.assign(settings.storage, originalSettings.value.storage)
    Object.assign(settings.security, originalSettings.value.security)
  }
  editing.value = false
}

async function saveSettings() {
  saving.value = true
  try {
    // 发送可修改的字段
    const payload: any = {}

    if (originalSettings.value) {
      // 存储配置
      if (settings.storage.region !== originalSettings.value.storage.region) {
        payload.region = settings.storage.region
      }
      if (settings.storage.max_object_size !== originalSettings.value.storage.max_object_size) {
        payload.max_object_size = settings.storage.max_object_size
      }
      if (settings.storage.max_upload_size !== originalSettings.value.storage.max_upload_size) {
        payload.max_upload_size = settings.storage.max_upload_size
      }
      // 安全配置
      if (settings.security.cors_origin !== originalSettings.value.security.cors_origin) {
        payload.cors_origin = settings.security.cors_origin
      }
      if (settings.security.presign_scheme !== originalSettings.value.security.presign_scheme) {
        payload.presign_scheme = settings.security.presign_scheme
      }
    }

    await axios.put(`${auth.endpoint}/api/admin/settings`, payload, {
      headers: getHeaders()
    })

    originalSettings.value = JSON.parse(JSON.stringify(settings))
    editing.value = false
    ElMessage.success('设置已保存')
  } catch (error: any) {
    ElMessage.error('保存失败: ' + (error.response?.data?.message || error.message))
  } finally {
    saving.value = false
  }
}

async function changePassword() {
  if (!passwordForm.oldPassword || !passwordForm.newPassword) {
    ElMessage.warning('请填写所有字段')
    return
  }
  if (passwordForm.newPassword !== passwordForm.confirmPassword) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }
  if (passwordForm.newPassword.length < 6) {
    ElMessage.warning('新密码至少6个字符')
    return
  }

  changingPassword.value = true
  try {
    await axios.post(`${auth.endpoint}/api/admin/settings/password`, {
      old_password: passwordForm.oldPassword,
      new_password: passwordForm.newPassword
    }, {
      headers: getHeaders()
    })
    
    ElMessage.success('密码修改成功')
    showPasswordDialog.value = false
    passwordForm.oldPassword = ''
    passwordForm.newPassword = ''
    passwordForm.confirmPassword = ''
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || '修改失败')
  } finally {
    changingPassword.value = false
  }
}

function formatDate(dateStr: string): string {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN')
}

onMounted(() => {
  loadSettings()
})
</script>

<style scoped>
.page-container {
  max-width: 1000px;
}

.page-header {
  margin-bottom: 20px;
}

.page-title h1 {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
  color: #333;
}

.page-subtitle {
  margin: 4px 0 0;
  font-size: 13px;
  color: #888;
}

.settings-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}

.settings-card {
  background: #fff;
  border: 1px solid #eee;
  border-radius: 10px;
  overflow: hidden;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 18px;
  border-bottom: 1px solid #f0f0f0;
  background: #fafafa;
}

.card-icon {
  font-size: 18px;
  color: #e67e22;
}

.card-header h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: #333;
}

.card-body {
  padding: 18px;
}

.setting-item {
  margin-bottom: 16px;
}

.setting-item:last-child {
  margin-bottom: 0;
}

.setting-item label {
  display: block;
  font-size: 13px;
  font-weight: 500;
  color: #666;
  margin-bottom: 6px;
}

.setting-hint {
  display: block;
  font-size: 12px;
  color: #999;
  margin-top: 4px;
}

.info-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 0;
  border-bottom: 1px solid #f5f5f5;
}

.info-item:last-child {
  border-bottom: none;
}

.info-label {
  color: #888;
  font-size: 13px;
}

.info-value {
  color: #333;
  font-size: 13px;
  font-weight: 500;
}

.info-value.mono {
  font-family: ui-monospace, monospace;
  font-size: 12px;
  color: #666;
}

.full-width-btn {
  width: 100%;
}

.action-bar {
  margin-top: 20px;
  display: flex;
  gap: 10px;
  justify-content: flex-end;
}

.primary-btn {
  background: #e67e22;
  border-color: #e67e22;
}

.primary-btn:hover {
  background: #d35400;
  border-color: #d35400;
}

/* 响应式 */
@media (max-width: 768px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }
}
</style>
