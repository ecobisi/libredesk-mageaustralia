<template>
  <AdminSplitLayout>
    <template #content>
      <div :class="{ 'opacity-50 transition-opacity duration-300': isLoading }" class="space-y-6">
        <form @submit.prevent="onSubmit" class="space-y-6 w-full max-w-xl">
          <div class="space-y-2">
            <Label for="pci-notify-agent">{{ t('admin.pci.notifyAgent') }}</Label>
            <Select v-model="notifyAgentId">
              <SelectTrigger id="pci-notify-agent" class="max-w-xs">
                <SelectValue :placeholder="t('admin.pci.notifyAgentPlaceholder')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem :value="0">{{ t('admin.pci.notifyAgentNone') }}</SelectItem>
                <SelectItem
                  v-for="agent in agents"
                  :key="agent.id"
                  :value="agent.id"
                >
                  {{ agent.first_name }} {{ agent.last_name }}
                </SelectItem>
              </SelectContent>
            </Select>
            <p class="text-xs text-muted-foreground">{{ t('admin.pci.notifyAgentDescription') }}</p>
          </div>

          <div class="space-y-2">
            <Label for="pci-notify-method">{{ t('admin.pci.notifyMethod') }}</Label>
            <Select v-model="notifyMethod">
              <SelectTrigger id="pci-notify-method" class="max-w-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="in_app">{{ t('admin.pci.notifyMethodInApp') }}</SelectItem>
                <SelectItem value="email">{{ t('admin.pci.notifyMethodEmail') }}</SelectItem>
                <SelectItem value="both">{{ t('admin.pci.notifyMethodBoth') }}</SelectItem>
              </SelectContent>
            </Select>
            <p class="text-xs text-muted-foreground">{{ t('admin.pci.notifyMethodDescription') }}</p>
          </div>

          <Button type="submit" :isLoading="saving">
            {{ t('globals.messages.save') }}
          </Button>
        </form>
        <Spinner v-if="isLoading" />
      </div>
    </template>
    <template #help>
      <p>{{ t('admin.pci.help') }}</p>
    </template>
  </AdminSplitLayout>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AdminSplitLayout from '@/layouts/admin/AdminSplitLayout.vue'
import { Button } from '@shared-ui/components/ui/button'
import { Label } from '@shared-ui/components/ui/label'
import { Spinner } from '@shared-ui/components/ui/spinner'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@shared-ui/components/ui/select'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import { useEmitter } from '@/composables/useEmitter'
import { EMITTER_EVENTS } from '@/constants/emitterEvents'
import api from '@/api'

const { t } = useI18n()
const emitter = useEmitter()

const isLoading = ref(true)
const saving = ref(false)
// notifyAgentId == 0 disables notifications entirely (server-side guard).
// notifyMethod default 'both' matches server-side default when the field
// is empty in DB; spelled out here so the Select shows a default selection.
const notifyAgentId = ref(0)
const notifyMethod = ref('both')
const agents = ref([])

const showToast = (description, variant) =>
  emitter.emit(EMITTER_EVENTS.SHOW_TOAST, variant ? { variant, description } : { description })

onMounted(async () => {
  try {
    const [settingsRes, agentsRes] = await Promise.all([
      api.getSettings('pci'),
      api.getUsersCompact()
    ])
    const data = settingsRes.data?.data || {}
    if (data['pci.notify_agent_id'] !== undefined) notifyAgentId.value = data['pci.notify_agent_id']
    if (data['pci.notify_method']) notifyMethod.value = data['pci.notify_method']
    agents.value = agentsRes.data?.data || []
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  } finally {
    isLoading.value = false
  }
})

const onSubmit = async () => {
  saving.value = true
  try {
    await api.updateSettings('pci', {
      'pci.notify_agent_id': Number(notifyAgentId.value) || 0,
      'pci.notify_method': notifyMethod.value || 'both'
    })
    showToast(t('globals.messages.savedSuccessfully'))
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  } finally {
    saving.value = false
  }
}
</script>
