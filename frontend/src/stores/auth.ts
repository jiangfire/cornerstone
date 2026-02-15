import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authAPI, userAPI } from '@/services/api'
import { ElMessage } from 'element-plus'

export interface User {
  id: string
  username: string
  email?: string
  role?: string
  phone?: string
  bio?: string
  avatar?: string
}

export interface AuthState {
  token: string | null
  user: User | null
  loading: boolean
}

export const useAuthStore = defineStore('auth', () => {
  // State
  const token = ref<string | null>(localStorage.getItem('auth_token'))
  const user = ref<User | null>(null)
  const loading = ref(false)

  // Getters
  const isAuthenticated = computed(() => !!token.value)
  const currentUser = computed(() => user.value)
  const username = computed(() => user.value?.username || '')

  // Actions
  const setToken = (newToken: string) => {
    token.value = newToken
    localStorage.setItem('auth_token', newToken)
  }

  const setUser = (newUser: User) => {
    user.value = newUser
    localStorage.setItem('user_info', JSON.stringify(newUser))
  }

  const clearAuth = () => {
    token.value = null
    user.value = null
    localStorage.removeItem('auth_token')
    localStorage.removeItem('user_info')
  }

  // 从 localStorage 恢复用户信息
  const restoreAuth = () => {
    const savedUser = localStorage.getItem('user_info')
    if (savedUser) {
      try {
        user.value = JSON.parse(savedUser)
      } catch {
        localStorage.removeItem('user_info')
      }
    }
  }

  // 登录
  const login = async (username: string, password: string) => {
    loading.value = true
    try {
      const response = await authAPI.login({ username, password })

      if (response.success && response.data?.token) {
        setToken(response.data.token)

        // 获取用户信息
        const profile = await userAPI.getProfile()
        if (profile.success && profile.data) {
          setUser(profile.data)
        }

        ElMessage.success('登录成功')
        return true
      }

      ElMessage.error(response.message || '登录失败')
      return false
    } catch (error: unknown) {
      ElMessage.error(error instanceof Error ? error.message : '登录失败，请检查网络连接')
      return false
    } finally {
      loading.value = false
    }
  }

  // 注册
  const register = async (userData: { username: string; email: string; password: string }) => {
    loading.value = true
    try {
      const response = await authAPI.register(userData)

      if (response.success) {
        ElMessage.success('注册成功，请登录')
        return true
      }

      ElMessage.error(response.message || '注册失败')
      return false
    } catch (error: unknown) {
      ElMessage.error(error instanceof Error ? error.message : '注册失败，请检查网络连接')
      return false
    } finally {
      loading.value = false
    }
  }

  // 登出
  const logout = async () => {
    try {
      // 调用后端登出接口（如果存在）
      if (token.value) {
        await authAPI.logout().catch(() => {
          // 忽略错误，因为token会在后端失效
        })
      }
    } finally {
      clearAuth()
      ElMessage.success('已退出登录')
    }
  }

  // 获取用户信息
  const fetchProfile = async () => {
    if (!token.value) return

    try {
      const response = await userAPI.getProfile()
      if (response.success && response.data) {
        setUser(response.data)
      }
    } catch {
      // 如果获取失败，可能是token过期，清除认证信息
      clearAuth()
    }
  }

  // 初始化
  const init = () => {
    restoreAuth()
    if (token.value && !user.value) {
      fetchProfile()
    }
  }

  return {
    // State
    token,
    user,
    loading,

    // Getters
    isAuthenticated,
    currentUser,
    username,

    // Actions
    login,
    register,
    logout,
    fetchProfile,
    clearAuth,
    init,
  }
})
