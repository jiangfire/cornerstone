<script setup lang="ts">
import { RouterView } from 'vue-router'
</script>

<template>
  <div id="app">
    <nav class="navbar">
      <RouterLink to="/" class="nav-link">数据浏览</RouterLink>
      <RouterLink to="/tokens" class="nav-link">令牌管理</RouterLink>
      <RouterLink to="/ai" class="nav-link">AI 助手</RouterLink>
    </nav>
    <main class="main-content">
      <RouterView />
    </main>
  </div>
</template>

<style>
:root {
  --primary: #1890ff;
  --bg: #f5f7fa;
  --text: #333;
  --border: #e8e8e8;
}

* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  color: var(--text);
  background: var(--bg);
}

#app {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.navbar {
  background: white;
  border-bottom: 1px solid var(--border);
  padding: 0 20px;
  display: flex;
  gap: 20px;
  height: 50px;
  align-items: center;
}

.nav-link {
  color: var(--text);
  text-decoration: none;
  padding: 8px 12px;
  border-radius: 4px;
  transition: background 0.2s;
}

.nav-link:hover {
  background: #f0f0f0;
}

.nav-link.router-link-active {
  background: var(--primary);
  color: white;
}

.main-content {
  flex: 1;
  overflow: auto;
}
</style>
