import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    // 认证相关路由（无需登录）
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
      meta: { requiresAuth: false, title: '登录' },
    },
    {
      path: '/register',
      name: 'register',
      component: () => import('../views/RegisterView.vue'),
      meta: { requiresAuth: false, title: '注册' },
    },

    // 主应用路由（需要登录）
    {
      path: '/',
      name: 'dashboard',
      component: () => import('../views/DashboardView.vue'),
      meta: { requiresAuth: true, title: '工作台' },
    },
    {
      path: '/organizations',
      name: 'organizations',
      component: () => import('../views/OrganizationsView.vue'),
      meta: { requiresAuth: true, title: '组织管理' },
    },
    {
      path: '/databases',
      name: 'databases',
      component: () => import('../views/DatabasesView.vue'),
      meta: { requiresAuth: true, title: '数据库管理' },
    },
    {
      path: '/databases/:id',
      name: 'tables',
      component: () => import('../views/TableView.vue'),
      meta: { requiresAuth: true, title: '表管理' },
    },
    {
      path: '/tables/:id/fields',
      name: 'fields',
      component: () => import('../views/FieldsView.vue'),
      meta: { requiresAuth: true, title: '字段管理' },
    },
    {
      path: '/tables/:id/records',
      name: 'records',
      component: () => import('../views/RecordsView.vue'),
      meta: { requiresAuth: true, title: '数据记录' },
    },
    {
      path: '/plugins',
      name: 'plugins',
      component: () => import('../views/PluginsView.vue'),
      meta: { requiresAuth: true, title: '插件管理' },
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('../views/SettingsView.vue'),
      meta: { requiresAuth: true, title: '系统设置' },
    },
    {
      path: '/profile',
      name: 'profile',
      component: () => import('../views/ProfileView.vue'),
      meta: { requiresAuth: true, title: '个人资料' },
    },

    // 404 路由
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('../views/NotFoundView.vue'),
    },
  ],
})

// 路由守卫
router.beforeEach((to, from, next) => {
  const authStore = useAuthStore()

  // 检查是否需要认证
  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    // 未登录，跳转到登录页并携带原目标路径
    next({
      path: '/login',
      query: { redirect: to.fullPath },
    })
    return
  }

  // 已登录但访问登录/注册页，跳转到首页
  if (to.path === '/login' || to.path === '/register') {
    if (authStore.isAuthenticated) {
      next('/')
      return
    }
  }

  // 设置页面标题
  if (to.meta.title) {
    document.title = `${to.meta.title} - Cornerstone`
  } else {
    document.title = 'Cornerstone'
  }

  next()
})

export default router
