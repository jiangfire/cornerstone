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
              <el-avatar :size="100" :src="profileForm.avatar" class="avatar">
                <UserFilled v-if="!profileForm.avatar" />
              </el-avatar>
              <div class="upload-actions">
                <el-upload
                  action="#"
                  :auto-upload="false"
                  :show-file-list="false"
                  :on-change="handleAvatarChange"
                  :limit="1"
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
            <el-button type="warning" @click="changePasswordDialog = true">
              修改密码
            </el-button>
            <el-button type="danger" @click="confirmDeleteAccount">
              删除账户
            </el-button>
          </div>
        </el-col>
      </el-row>
    </el-card>

    <!-- 修改密码对话框 -->
    <el-dialog
      v-model="changePasswordDialog"
      title="修改密码"
      width="400px"
    >
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
import { ElMessage, ElMessageBox } from 'element-plus'
import { UserFilled } from '@element-plus/icons-vue'
import type { UploadFile } from 'element-plus'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()

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
    // 从 auth store 获取用户信息
    const user = authStore.user
    if (user) {
      profileForm.value = {
        username: user.username || '',
        email: user.email || '',
        phone: user.phone || '',
        bio: user.bio || '',
        avatar: user.avatar || '',
      }
    }
  } catch (error) {
    ElMessage.error('加载个人资料失败')
  }
}

const updateProfile = async () => {
  updating.value = true
  try {
    // 调用 API 更新个人资料
    await new Promise(resolve => setTimeout(resolve, 1000))
    ElMessage.success('个人资料更新成功')
    await loadProfile()
  } catch (error) {
    ElMessage.error('更新失败')
  } finally {
    updating.value = false
  }
}

const handleAvatarChange = (file: UploadFile) => {
  // 模拟头像上传
  const reader = new FileReader()
  reader.onload = (e) => {
    if (e.target?.result) {
      profileForm.value.avatar = e.target.result as string
      ElMessage.success('头像已更新')
    }
  }
  if (file.raw) {
    reader.readAsDataURL(file.raw)
  }
}

const removeAvatar = () => {
  profileForm.value.avatar = ''
  ElMessage.success('头像已移除')
}

const changePassword = async () => {
  if (passwordForm.value.newPassword !== passwordForm.value.confirmPassword) {
    ElMessage.error('两次输入的密码不一致')
    return
  }

  changingPassword.value = true
  try {
    // 调用 API 修改密码
    await new Promise(resolve => setTimeout(resolve, 1000))
    ElMessage.success('密码修改成功')
    changePasswordDialog.value = false
    passwordForm.value = {
      currentPassword: '',
      newPassword: '',
      confirmPassword: '',
    }
  } catch (error) {
    ElMessage.error('密码修改失败')
  } finally {
    changingPassword.value = false
  }
}

const confirmDeleteAccount = () => {
  ElMessageBox.confirm(
    '确定要删除账户吗？此操作不可恢复，所有相关数据将被清空。',
    '警告',
    {
      type: 'warning',
      confirmButtonText: '确定删除',
      cancelButtonText: '取消',
    }
  ).then(() => {
    deleteAccount()
  }).catch(() => {})
}

const deleteAccount = async () => {
  try {
    // 调用 API 删除账户
    await new Promise(resolve => setTimeout(resolve, 1000))
    ElMessage.success('账户已删除')
    // 退出登录
    await authStore.logout()
  } catch (error) {
    ElMessage.error('删除账户失败')
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