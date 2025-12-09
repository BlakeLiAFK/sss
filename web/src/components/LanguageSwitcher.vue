<template>
  <el-dropdown trigger="click" @command="handleCommand">
    <el-button :class="buttonClass" :size="size">
      <el-icon><i-ep-global /></el-icon>
      <span v-if="showLabel" class="lang-label">{{ currentLabel }}</span>
      <el-icon class="el-icon--right"><arrow-down /></el-icon>
    </el-button>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item command="zh-CN" :class="{ active: locale === 'zh-CN' }">
          <span class="lang-option">{{ t('language.zhCN') }}</span>
          <el-icon v-if="locale === 'zh-CN'" class="check-icon"><Check /></el-icon>
        </el-dropdown-item>
        <el-dropdown-item command="en-US" :class="{ active: locale === 'en-US' }">
          <span class="lang-option">{{ t('language.enUS') }}</span>
          <el-icon v-if="locale === 'en-US'" class="check-icon"><Check /></el-icon>
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowDown, Check } from '@element-plus/icons-vue'
import { setLocale } from '../i18n'

const props = withDefaults(defineProps<{
  showLabel?: boolean
  size?: 'small' | 'default' | 'large'
  buttonClass?: string
}>(), {
  showLabel: true,
  size: 'default',
  buttonClass: ''
})

const { t, locale } = useI18n()

const currentLabel = computed(() => {
  return locale.value === 'zh-CN' ? '中文' : 'EN'
})

function handleCommand(lang: string) {
  setLocale(lang)
}
</script>

<style scoped>
.lang-label {
  margin-left: 4px;
}

:deep(.el-dropdown-menu__item) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-width: 120px;
}

:deep(.el-dropdown-menu__item.active) {
  color: #e67e22;
  background-color: rgba(230, 126, 34, 0.1);
}

.lang-option {
  flex: 1;
}

.check-icon {
  margin-left: 8px;
  color: #e67e22;
}
</style>
