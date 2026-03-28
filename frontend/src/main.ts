import './assets/main.css'

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import 'element-plus/dist/index.css'
import {
  ElAlert,
  ElAvatar,
  ElButton,
  ElCard,
  ElCheckbox,
  ElCol,
  ElDatePicker,
  ElDialog,
  ElDivider,
  ElDrawer,
  ElDropdown,
  ElDropdownItem,
  ElDropdownMenu,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElIcon,
  ElInput,
  ElInputNumber,
  ElLink,
  ElLoading,
  ElMenu,
  ElMenuItem,
  ElOption,
  ElPagination,
  ElProgress,
  ElResult,
  ElRow,
  ElSelect,
  ElSwitch,
  ElTabPane,
  ElTable,
  ElTableColumn,
  ElTabs,
  ElTag,
  ElTimeline,
  ElTimelineItem,
  ElUpload,
} from 'element-plus'

import App from './App.vue'
import router from './router'

const app = createApp(App)

const elementComponents = {
  ElAlert,
  ElAvatar,
  ElButton,
  ElCard,
  ElCheckbox,
  ElCol,
  ElDatePicker,
  ElDialog,
  ElDivider,
  ElDrawer,
  ElDropdown,
  ElDropdownItem,
  ElDropdownMenu,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElIcon,
  ElInput,
  ElInputNumber,
  ElLink,
  ElMenu,
  ElMenuItem,
  ElOption,
  ElPagination,
  ElProgress,
  ElResult,
  ElRow,
  ElSelect,
  ElSwitch,
  ElTabPane,
  ElTable,
  ElTableColumn,
  ElTabs,
  ElTag,
  ElTimeline,
  ElTimelineItem,
  ElUpload,
}

for (const [name, component] of Object.entries(elementComponents)) {
  app.component(name, component)
}

app.directive('loading', ElLoading.directive)

app.use(createPinia())
app.use(router)

app.mount('#app')
