import { computed } from 'vue'
import { useRoute } from 'vue-router'

export function usePageMeta() {
  const route = useRoute()
  const title = computed(() => (route.meta.title as string) || 'Cornerstone')
  return { title }
}
