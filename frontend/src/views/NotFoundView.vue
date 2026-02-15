<template>
  <div class="not-found">
    <div class="not-found-card">
      <div class="not-found-icon">404</div>
      <h2>页面未找到</h2>
      <p>您访问的页面不存在或已被移除</p>
      <div class="not-found-actions">
        <el-button type="primary" @click="$router.push('/')">返回首页</el-button>
        <el-button @click="$router.back()">返回上一页</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts"></script>

<style scoped lang="scss">
.not-found {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 60vh;
}

.not-found-card {
  text-align: center;
  background: var(--fa-bg-surface);
  -webkit-backdrop-filter: var(--fa-blur);
  backdrop-filter: var(--fa-blur);
  border: var(--fa-border);
  border-radius: var(--fa-radius-xl);
  box-shadow: var(--fa-shadow-md);
  padding: 48px 64px;
  max-width: 480px;

  .not-found-icon {
    font-size: 72px;
    font-weight: 700;
    background: var(--fa-accent);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
    line-height: 1;
    margin-bottom: 16px;
  }

  h2 {
    font-size: 22px;
    font-weight: 600;
    color: var(--fa-text-primary);
    margin: 0 0 8px;
  }

  p {
    color: var(--fa-text-muted);
    font-size: 14px;
    margin: 0 0 24px;
  }

  .not-found-actions {
    display: flex;
    gap: 12px;
    justify-content: center;
  }
}
</style>
