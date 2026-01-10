<template>
  <div class="auth-container">
    <div class="auth-card">
      <div class="auth-header">
        <h2>登录</h2>
        <p class="subtitle">欢迎回到 Cornerstone 硬件工程数据平台</p>
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
            @keyup.enter="handleSubmit"
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
            @keyup.enter="handleSubmit"
          />
        </el-form-item>

        <el-form-item>
          <div class="form-options">
            <el-checkbox v-model="form.remember">记住我</el-checkbox>
            <el-link type="primary" @click="$router.push('/register')">
              没有账号？立即注册
            </el-link>
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
            登录
          </el-button>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { User, Lock } from '@element-plus/icons-vue'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage } from 'element-plus'

const router = useRouter()
const authStore = useAuthStore()

const formRef = ref<FormInstance>()
const form = reactive({
  username: '',
  password: '',
  remember: true,
})

const rules: FormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 20, message: '用户名长度应在 3-20 个字符之间', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, max: 20, message: '密码长度应在 6-20 个字符之间', trigger: 'blur' },
  ],
}

onMounted(() => {
  // 如果已经登录，直接跳转到首页
  if (authStore.isAuthenticated) {
    router.push('/')
  }
})

const handleSubmit = async () => {
  if (!formRef.value) return

  try {
    const valid = await formRef.value.validate()
    if (!valid) return

    const success = await authStore.login(form.username, form.password)
    if (success) {
      // 登录成功后跳转到首页或之前访问的页面
      const redirect = router.currentRoute.value.query.redirect as string
      router.push(redirect || '/')
    }
  } catch (error) {
    console.error('登录验证失败:', error)
  }
}
</script>

<style scoped lang="scss">
.auth-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 20px;
}

.auth-card {
  background: white;
  border-radius: 12px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.1);
  padding: 40px;
  width: 100%;
  max-width: 400px;
}

.auth-header {
  text-align: center;
  margin-bottom: 32px;

  h2 {
    margin: 0 0 8px;
    font-size: 28px;
    font-weight: 600;
    color: #303133;
  }

  .subtitle {
    margin: 0;
    color: #909399;
    font-size: 14px;
  }
}

.auth-form {
  :deep(.el-form-item__label) {
    font-weight: 500;
    margin-bottom: 8px;
  }

  .form-options {
    display: flex;
    justify-content: space-between;
    align-items: center;
    width: 100%;
    margin-top: -8px;
  }

  .submit-btn {
    width: 100%;
    font-weight: 600;
    margin-top: 8px;
  }
}
</style>