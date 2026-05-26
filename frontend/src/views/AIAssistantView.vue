<template>
  <div class="ai-assistant">
    <div class="header">
      <h1>AI 助手</h1>
    </div>

    <div class="chat-container" ref="chatContainer">
      <div v-for="(msg, index) in messages" :key="index" :class="['message', msg.role]">
        <div class="message-content">
          <strong>{{ msg.role === 'user' ? '你' : 'AI' }}:</strong>
          <div class="text" v-if="msg.role === 'user'">{{ msg.content }}</div>
          <div class="text" v-else v-html="formatMarkdown(msg.content)"></div>
        </div>
      </div>
      <div v-if="loading" class="message ai">
        <div class="message-content">
          <strong>AI:</strong>
          <div class="text thinking">思考中...</div>
        </div>
      </div>
    </div>

    <div class="input-area">
      <textarea v-model="inputMessage" @keydown.enter.exact.prevent="sendMessage" placeholder="输入消息... (Enter 发送)" rows="3"></textarea>
      <button class="btn btn-primary" @click="sendMessage" :disabled="loading || !inputMessage.trim()">发送</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick } from 'vue'
import { aiAPI } from '@/services/api'
import type { AxiosError } from 'axios'

interface Message {
  role: 'user' | 'ai'
  content: string
}

const messages = ref<Message[]>([])
const inputMessage = ref('')
const loading = ref(false)
const chatContainer = ref<HTMLElement | null>(null)

async function sendMessage() {
  if (!inputMessage.value.trim() || loading.value) return

  const userMsg = inputMessage.value.trim()
  messages.value.push({ role: 'user', content: userMsg })
  inputMessage.value = ''
  loading.value = true

  await scrollToBottom()

  try {
    const res = await aiAPI.chat(userMsg)
    messages.value.push({ role: 'ai', content: res.data.data.reply || '暂无回复' })
  } catch (e) {
    const err = e as AxiosError<{ message?: string }>
    const errorMsg = err.response?.data?.message || err.message || '请求失败'
    messages.value.push({ role: 'ai', content: `错误: ${errorMsg}` })
  } finally {
    loading.value = false
    await scrollToBottom()
  }
}

async function scrollToBottom() {
  await nextTick()
  if (chatContainer.value) {
    chatContainer.value.scrollTop = chatContainer.value.scrollHeight
  }
}

function formatMarkdown(text: string): string {
  if (!text) return ''
  const html = text
    .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.*?)\*/g, '<em>$1</em>')
    .replace(/`(.*?)`/g, '<code>$1</code>')
    .replace(/\n/g, '<br>')
  return html
}
</script>

<style scoped>
.ai-assistant {
  padding: 20px;
  height: 100vh;
  display: flex;
  flex-direction: column;
}
.header {
  margin-bottom: 20px;
}
.chat-container {
  flex: 1;
  overflow-y: auto;
  padding: 10px;
  background: #f9f9f9;
  border-radius: 8px;
  margin-bottom: 15px;
}
.message {
  margin-bottom: 15px;
  display: flex;
}
.message.user {
  justify-content: flex-end;
}
.message.ai {
  justify-content: flex-start;
}
.message-content {
  max-width: 70%;
  padding: 10px 15px;
  border-radius: 8px;
  background: white;
  box-shadow: 0 1px 3px rgba(0,0,0,0.1);
}
.message.user .message-content {
  background: #1890ff;
  color: white;
}
.text {
  margin-top: 5px;
  white-space: pre-wrap;
  word-break: break-word;
}
.thinking {
  color: #999;
  font-style: italic;
}
.input-area {
  display: flex;
  gap: 10px;
}
textarea {
  flex: 1;
  padding: 10px;
  border: 1px solid #ddd;
  border-radius: 4px;
  resize: none;
  font-family: inherit;
}
.btn {
  padding: 10px 20px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  align-self: flex-end;
}
.btn-primary {
  background: #1890ff;
  color: white;
}
.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
