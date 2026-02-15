import axios, { type AxiosInstance, type AxiosResponse, type AxiosError } from 'axios'
import type {
  ApiResponse,
  AuthResponse,
  LoginResponse,
  RegisterResponse,
  UserProfile,
  UserListResponse,
  DatabaseListResponse,
  TableListResponse,
  FieldListResponse,
  RecordListResponse,
  OrganizationListResponse,
  FileListResponse,
  PluginListResponse,
  StatsSummary,
} from '@/types/api'

// API 配置
const API_CONFIG = {
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: 10000,
}

// 创建 Axios 实例
const api: AxiosInstance = axios.create(API_CONFIG)

// 请求拦截器
api.interceptors.request.use(
  (config) => {
    // 从 localStorage 获取 token
    const token = localStorage.getItem('auth_token')
    if (token) {
      config.headers = { ...config.headers, Authorization: `Bearer ${token}` }
    }

    // 添加请求时间戳
    config.params = {
      ...config.params,
      _t: Date.now(),
    }

    return config
  },
  (error) => {
    return Promise.reject(error)
  },
)

// 响应拦截器
api.interceptors.response.use(
  (response: AxiosResponse) => {
    return response.data
  },
  (error: AxiosError) => {
    const { response } = error
    if (response?.status === 401) {
      // Token 过期或无效，清除本地存储并跳转到登录页
      localStorage.removeItem('auth_token')
      localStorage.removeItem('user_info')
      window.location.href = '/login'
    }

    return Promise.reject(error)
  },
)

// 通用请求方法
export const request = {
  get<T = unknown>(url: string, params?: Record<string, unknown>): Promise<ApiResponse<T>> => {
    return api.get(url, { params })
  },

  post<T = unknown>(url: string, data?: Record<string, unknown>): Promise<ApiResponse<T>> => {
    return api.post(url, data)
  },

  put<T = unknown>(url: string, data?: Record<string, unknown>): Promise<ApiResponse<T>> => {
    return api.put(url, data)
  },

  delete<T = unknown>(url: string, data?: Record<string, unknown>): Promise<ApiResponse<T>> => {
    return data ? api.delete(url, { data }) : api.delete(url)
  },
}
