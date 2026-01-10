<template>
  <div class="not-found">
    <el-result
      icon="warning"
      title="404 - 页面未找到"
      sub-title="您访问的页面不存在或已被移除"
    >
      <template #extra>
        <el-button type="primary" @click="$router.push('/')">返回首页</el-button>
        <el-button @click="$router.back()">返回上一页</el-button>
      </template>
    </el-result>
  </div>
</template>

<script setup lang="ts">
// 404 页面组件 - 简单直接
</script>

<style scoped lang="scss">
.not-found {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  min-height: 400px;
}
</style>