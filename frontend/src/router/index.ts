import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'data-browser',
      component: () => import('../views/DataBrowserView.vue'),
      meta: { title: '数据浏览' },
    },
    {
      path: '/tokens',
      name: 'tokens',
      component: () => import('../views/TokensView.vue'),
      meta: { title: '令牌管理' },
    },
    {
      path: '/ai',
      name: 'ai-assistant',
      component: () => import('../views/AIAssistantView.vue'),
      meta: { title: 'AI 助手' },
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('../views/NotFoundView.vue'),
    },
  ],
})

router.beforeEach((to, from, next) => {
  if (to.meta.title) {
    document.title = `${to.meta.title} - Cornerstone`
  } else {
    document.title = 'Cornerstone'
  }
  next()
})

export default router
