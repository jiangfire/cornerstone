<template>
  <div class="auth-card">
    <div class="auth-header">
      <div class="auth-logo">C</div>
      <h2>注册账号</h2>
      <p class="subtitle">加入 Cornerstone 数据平台</p>
    </div>

    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      :loading="authStore.loading"
      class="auth-form"
      @submit.prevent="handleSubmit"
    >
      <el-form-item prop="username" label="用户名">
        <el-input
          v-model="form.username"
          placeholder="请输入用户名"
          size="large"
          :prefix-icon="User"
        />
      </el-form-item>

      <el-form-item prop="email" label="邮箱">
        <el-input
          v-model="form.email"
          placeholder="请输入邮箱"
          size="large"
          :prefix-icon="Message"
        />
      </el-form-item>

      <el-form-item prop="password" label="密码">
        <el-input
          v-model="form.password"
          type="password"
          placeholder="请输入密码"
          size="large"
          :prefix-icon="Lock"
          show-password
        />
      </el-form-item>

      <el-form-item prop="confirmPassword" label="确认密码">
        <el-input
          v-model="form.confirmPassword"
          type="password"
          placeholder="请再次输入密码"
          size="large"
          :prefix-icon="Lock"
          show-password
        />
      </el-form-item>

      <el-form-item>
        <div class="form-options">
          <el-checkbox v-model="form.agree">
            我已阅读并同意
            <el-link type="primary" :underline="false">服务条款</el-link>
            和
            <el-link type="primary" :underline="false">隐私政策</el-link>
          </el-checkbox>
          <el-link type="primary" @click="$router.push('/login')"> 已有账号？立即登录 </el-link>
        </div>
      </el-form-item>

      <el-form-item>
        <el-button
          type="primary"
          size="large"
          native-type="submit"
          :loading="authStore.loading"
          class="submit-btn"
        >
          注册
        </el-button>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { User, Lock, Message } from '@element-plus/icons-vue'
import type { FormInstance, FormRules } from 'element-plus'

const router = useRouter()
const authStore = useAuthStore()

const formRef = ref<FormInstance>()
const form = reactive({
  username: '',
  email: '',
  password: '',
  confirmPassword: '',
  agree: false,
})

const validatePass = (rule: unknown, value: string, callback: (error?: Error) => void) => {
  if (value === '') {
    callback(new Error('请输入密码'))
  } else {
    if (form.confirmPassword !== '') {
      if (!formRef.value) return
      formRef.value.validateField('confirmPassword')
    }
    callback()
  }
}

const validatePass2 = (rule: unknown, value: string, callback: (error?: Error) => void) => {
  if (value === '') {
    callback(new Error('请再次输入密码'))
  } else if (value !== form.password) {
    callback(new Error('两次输入的密码不一致！'))
  } else {
    callback()
  }
}

const rules: FormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 20, message: '用户名长度应在 3-20 个字符之间', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_]+$/, message: '用户名只能包含字母、数字和下划线', trigger: 'blur' },
  ],
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '请输入正确的邮箱格式', trigger: 'blur' },
  ],
  password: [
    { required: true, validator: validatePass, trigger: 'blur' },
    { min: 6, max: 20, message: '密码长度应在 6-20 个字符之间', trigger: 'blur' },
  ],
  confirmPassword: [{ required: true, validator: validatePass2, trigger: 'blur' }],
  agree: [
    {
      validator: (_rule: unknown, value: boolean, callback: (error?: Error) => void) => {
        if (!value) {
          callback(new Error('请同意服务条款和隐私政策'))
        } else {
          callback()
        }
      },
      trigger: 'change',
    },
  ],
}

const handleSubmit = async () => {
  if (!formRef.value) return

  try {
    const valid = await formRef.value.validate()
    if (!valid) return

    const success = await authStore.register({
      username: form.username,
      email: form.email,
      password: form.password,
    })

    if (success) {
      router.push('/login')
    }
  } catch (error) {
    console.error('注册验证失败:', error)
  }
}
</script>

<style scoped lang="scss">
.auth-card {
  background: var(--fa-bg-elevated);
  -webkit-backdrop-filter: var(--fa-blur-heavy);
  backdrop-filter: var(--fa-blur-heavy);
  border: var(--fa-border);
  border-radius: var(--fa-radius-xl);
  box-shadow: var(--fa-shadow-lg), var(--fa-shadow-glow);
  padding: 40px;
  width: 100%;
  max-width: 440px;
}

.auth-header {
  text-align: center;
  margin-bottom: 32px;

  .auth-logo {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 48px;
    height: 48px;
    border-radius: var(--fa-radius-md);
    background: var(--fa-accent);
    color: #fff;
    font-weight: 700;
    font-size: 22px;
    margin-bottom: 16px;
  }

  h2 {
    margin: 0 0 8px;
    font-size: 26px;
    font-weight: 600;
    color: var(--fa-text-primary);
  }

  .subtitle {
    margin: 0;
    color: var(--fa-text-muted);
    font-size: 14px;
  }
}

.auth-form {
  :deep(.el-form-item__label) {
    font-weight: 500;
    margin-bottom: 8px;
    color: var(--fa-text-secondary);
  }

  :deep(.el-input__wrapper) {
    background: rgba(255, 255, 255, 0.5);
    border-radius: var(--fa-radius-sm);
  }

  .form-options {
    display: flex;
    flex-direction: column;
    gap: 8px;
    width: 100%;
    margin-top: -8px;

    .el-checkbox {
      margin-right: 0;
    }
  }

  .submit-btn {
    width: 100%;
    font-weight: 600;
    margin-top: 8px;
    border-radius: var(--fa-radius-full);
    background: var(--fa-accent);
    border: none;
    height: 44px;
    font-size: 15px;

    &:hover {
      background: var(--fa-accent-hover);
    }
  }
}
</style>
