<template>
  <div>
    <div class="flex items-center px-2 h-12">
      <SidebarTrigger class="cursor-pointer" />
      <Input
        ref="searchInput"
        v-model="model"
        :placeholder="t('globals.terms.search')"
        class="w-full border-none shadow-none focus:ring-0 focus:ring-offset-0"
      />
    </div>
    <Separator />
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue'
import { Separator } from '@shared-ui/components/ui/separator'
import { Input } from '@shared-ui/components/ui/input'
import { SidebarTrigger } from '@shared-ui/components/ui/sidebar'
import { useI18n } from 'vue-i18n'
const model = defineModel({
  type: String,
  default: '',
  required: false
})
const { t } = useI18n()

// FS8: Auto-focus the search input when the page loads. Lands the cursor in
// the input so the user can start typing immediately. The shadcn Input wraps
// a real <input> element; if the ref unwraps to the wrapper, we fall back to
// querying the underlying input.
const searchInput = ref(null)
const focusInput = () => {
  const el = searchInput.value?.$el ?? searchInput.value
  if (!el) return
  if (typeof el.focus === 'function') {
    el.focus()
    return
  }
  el.querySelector?.('input')?.focus()
}
onMounted(() => {
  nextTick(focusInput)
})

// FS14: Exposed so SearchView can refocus the input after the sidebar's
// search icon clears state on a same-route navigation (no remount).
defineExpose({ focus: focusInput })
</script>

<style scoped>
.focus\:ring-0:focus {
  --tw-ring-offset-shadow: none;
  --tw-ring-shadow: none;
  box-shadow: none;
}
</style>
