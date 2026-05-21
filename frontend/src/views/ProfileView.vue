<template>
  <div class="profile">
    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <span>个人资料</span>
        </div>
      </template>

      <el-row :gutter="20">
        <el-col :span="12">
          <el-form :model="profileForm" label-width="100px" ref="profileFormRef">
            <el-form-item label="用户名" prop="username">
              <el-input v-model="profileForm.username" placeholder="请输入用户名" />
            </el-form-item>
            <el-form-item label="邮箱" prop="email">
              <el-input v-model="profileForm.email" placeholder="请输入邮箱" />
            </el-form-item>
            <el-form-item label="手机号" prop="phone">
              <el-input v-model="profileForm.phone" placeholder="请输入手机号" />
            </el-form-item>
            <el-form-item label="简介" prop="bio">
              <el-input
                v-model="profileForm.bio"
                type="textarea"
                :rows="3"
                placeholder="个人简介"
              />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="updateProfile" :loading="updating">
                更新资料
              </el-button>
            </el-form-item>
          </el-form>
        </el-col>

        <el-col :span="12">
          <div class="avatar-section">
            <h4>头像</h4>
            <div class="avatar-upload">
              <el-avatar :size="100" :src="avatarURL(profileForm.avatar)" class="avatar">
                <UserFilled v-if="!profileForm.avatar" />
              </el-avatar>
              <div class="upload-actions">
                <el-upload
                  action="#"
                  :auto-upload="false"
                  :show-file-list="false"
                  :on-change="handleAvatarChange"
                  :limit="1"
                  accept="image/png,image/jpeg,image/webp"
                >
                  <el-button size="small" type="primary">更换头像</el-button>
                </el-upload>
                <el-button size="small" @click="removeAvatar" v-if="profileForm.avatar">
                  移除头像
                </el-button>
              </div>
            </div>
          </div>

          <div class="security-section">
            <h4>安全设置</h4>
            <el-button type="warning" @click="changePasswordDialog = true"> 修改密码 </el-button>
            <el-button type="danger" @click="confirmDeleteAccount"> 删除账户 </el-button>
          </div>
        </el-col>
      </el-row>
    </el-card>

    <!-- 修改密码对话框 -->
    <el-dialog v-model="changePasswordDialog" title="修改密码" width="400px">
      <el-form :model="passwordForm" label-width="100px" ref="passwordFormRef">
        <el-form-item label="当前密码" prop="currentPassword">
          <el-input
            v-model="passwordForm.currentPassword"
            type="password"
            placeholder="请输入当前密码"
            show-password
          />
        </el-form-item>
        <el-form-item label="新密码" prop="newPassword">
          <el-input
            v-model="passwordForm.newPassword"
            type="password"
            placeholder="请输入新密码"
            show-password
          />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirmPassword">
          <el-input
            v-model="passwordForm.confirmPassword"
            type="password"
            placeholder="请再次输入新密码"
            show-password
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="changePasswordDialog = false">取消</el-button>
          <el-button type="primary" @click="changePassword" :loading="changingPassword">
            确认修改
          </el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { UserFilled } from '@element-plus/icons-vue'
import type { UploadFile } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { userAPI, avatarAPI, avatarURL } from '@/services/api'

const authStore = useAuthStore()
const router = useRouter()

const updating = ref(false)
const changingPassword = ref(false)
const changePasswordDialog = ref(false)

const profileForm = ref({
  username: '',
  email: '',
  phone: '',
  bio: '',
  avatar: '',
})

const passwordForm = ref({
  currentPassword: '',
  newPassword: '',
  confirmPassword: '',
})

const loadProfile = async () => {
  try {
    const response = await userAPI.getProfile()
    if (response.code === 0 && response.data) {
      const user = response.data
      profileForm.value = {
        username: user.username || '',
        email: user.email || '',
        phone: user.phone || '',
        bio: user.bio || '',
        avatar: user.avatar || '',
      }
    }
  } catch {
    ElMessage.error('加载个人资料失败')
  }
}

const updateProfile = async () => {
  updating.value = true
  try {
    const response = await userAPI.updateProfile({
      username: profileForm.value.username,
      email: profileForm.value.email,
      phone: profileForm.value.phone,
      bio: profileForm.value.bio,
      avatar: profileForm.value.avatar,
    })
    if (response.code !== 0) {
      throw new Error(response.message || '更新失败')
    }
    ElMessage.success('个人资料更新成功')
    await authStore.fetchProfile()
    await loadProfile()
  } catch (error: unknown) {
    ElMessage.error(error instanceof Error ? error.message : '更新失败')
  } finally {
    updating.value = false
  }
}

const AVATAR_MAX_BYTES = 2 * 1024 * 1024 // 2MB
const AVATAR_ACCEPTED_MIME = new Set(['image/png', 'image/jpeg', 'image/webp', 'image/gif'])

const handleAvatarChange = async (file: UploadFile) => {
  const raw = file.raw
  if (!raw) {
    return
  }

  if (!AVATAR_ACCEPTED_MIME.has(raw.type)) {
    ElMessage.error('仅支持 PNG / JPEG / WebP / GIF 格式')
    return
  }

  if (raw.size > AVATAR_MAX_BYTES) {
    ElMessage.error('头像文件不能超过 2MB')
    return
  }

  try {
    const response = await avatarAPI.upload(raw)
    if (response.code !== 0 || !response.data?.avatar_url) {
      throw new Error(response.message || '上传失败')
    }
    profileForm.value.avatar = response.data.avatar_url
    ElMessage.success('头像上传成功')
    // 同步到后端用户资料（头像字段已持久化，但刷新 auth store 保证全局一致）
    await authStore.fetchProfile()
  } catch (error: unknown) {
    ElMessage.error(error instanceof Error ? error.message : '头像上传失败')
  }
}

const removeAvatar = async () => {
  profileForm.value.avatar = ''
  try {
    const response = await userAPI.updateProfile({
      username: profileForm.value.username,
      email: profileForm.value.email,
      phone: profileForm.value.phone,
      bio: profileForm.value.bio,
      avatar: '',
    })
    if (response.code !== 0) {
      throw new Error(response.message || '移除失败')
    }
    ElMessage.success('头像已移除')
    await authStore.fetchProfile()
  } catch (error: unknown) {
    ElMessage.error(error instanceof Error ? error.message : '移除头像失败')
  }
}

const changePassword = async () => {
  if (passwordForm.value.newPassword !== passwordForm.value.confirmPassword) {
    ElMessage.error('两次输入的密码不一致')
    return
  }

  changingPassword.value = true
  try {
    const response = await userAPI.changePassword({
      current_password: passwordForm.value.currentPassword,
      new_password: passwordForm.value.newPassword,
    })
    if (response.code !== 0) {
      throw new Error(response.message || '密码修改失败')
    }
    ElMessage.success('密码修改成功')
    changePasswordDialog.value = false
    passwordForm.value = {
      currentPassword: '',
      newPassword: '',
      confirmPassword: '',
    }
  } catch (error: unknown) {
    ElMessage.error(error instanceof Error ? error.message : '密码修改失败')
  } finally {
    changingPassword.value = false
  }
}

const confirmDeleteAccount = () => {
  ElMessageBox.confirm('确定要删除账户吗？此操作不可恢复，所有相关数据将被清空。', '警告', {
    type: 'warning',
    confirmButtonText: '确定删除',
    cancelButtonText: '取消',
  })
    .then(() => {
      deleteAccount()
    })
    .catch(() => {})
}

const deleteAccount = async () => {
  try {
    const { value } = await ElMessageBox.prompt('请输入当前密码以确认删除账户', '二次确认', {
      inputType: 'password',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      inputPlaceholder: '当前密码',
      closeOnClickModal: false,
    })

    const response = await userAPI.deleteAccount({ password: value })
    if (response.code !== 0) {
      throw new Error(response.message || '删除账户失败')
    }

    ElMessage.success('账户已删除')
    authStore.clearAuth()
    await router.replace('/login')
  } catch (error: unknown) {
    if (error !== 'cancel') {
      ElMessage.error(error instanceof Error ? error.message : '删除账户失败')
    }
  }
}

onMounted(() => {
  loadProfile()
})
</script>

<style scoped lang="scss">
.profile {
  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .avatar-section {
    margin-bottom: 24px;

    h4 {
      margin-bottom: 12px;
      font-size: 16px;
      font-weight: 600;
    }

    .avatar-upload {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 12px;

      .avatar {
        border: 2px solid #dcdfe6;
      }

      .upload-actions {
        display: flex;
        gap: 8px;
      }
    }
  }

  .security-section {
    h4 {
      margin-bottom: 12px;
      font-size: 16px;
      font-weight: 600;
    }

    .el-button {
      margin-right: 8px;
      margin-bottom: 8px;
    }
  }

  .dialog-footer {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
  }
}
</style>
