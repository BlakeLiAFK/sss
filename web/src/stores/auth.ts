import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  const accessKeyId = ref(localStorage.getItem('accessKeyId') || '')
  const secretAccessKey = ref(localStorage.getItem('secretAccessKey') || '')
  const endpoint = ref(localStorage.getItem('endpoint') || 'http://localhost:9000')
  const region = ref(localStorage.getItem('region') || 'us-east-1')

  const isLoggedIn = computed(() => accessKeyId.value !== '' && secretAccessKey.value !== '')

  function login(akId: string, sakKey: string, ep: string, reg: string) {
    accessKeyId.value = akId
    secretAccessKey.value = sakKey
    endpoint.value = ep
    region.value = reg
    localStorage.setItem('accessKeyId', akId)
    localStorage.setItem('secretAccessKey', sakKey)
    localStorage.setItem('endpoint', ep)
    localStorage.setItem('region', reg)
  }

  function logout() {
    accessKeyId.value = ''
    secretAccessKey.value = ''
    localStorage.removeItem('accessKeyId')
    localStorage.removeItem('secretAccessKey')
  }

  return { accessKeyId, secretAccessKey, endpoint, region, isLoggedIn, login, logout }
})
