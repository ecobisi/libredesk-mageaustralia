<template>
  <AdminSplitLayout>
    <template #content>
      <div :class="{ 'opacity-50 transition-opacity duration-300': isLoading }" class="space-y-6">
        <form @submit.prevent="onSubmit" class="space-y-6 w-full max-w-xl">
          <h2 class="text-base font-medium">{{ t('admin.ai.transcription.title') }}</h2>

          <div class="flex items-start justify-between gap-4">
            <div class="space-y-1">
              <Label for="ai-transcription-enabled">{{ t('admin.ai.transcription.enable') }}</Label>
              <p class="text-xs text-muted-foreground">{{ t('admin.ai.transcription.enableDescription') }}</p>
            </div>
            <Switch
              id="ai-transcription-enabled"
              v-model:checked="transcriptionEnabled"
            />
          </div>

          <div v-if="transcriptionEnabled" class="space-y-4">
            <div class="space-y-2">
              <Label for="ai-transcription-provider">{{ t('admin.ai.transcription.provider') }}</Label>
              <Select v-model="transcriptionProvider">
                <SelectTrigger id="ai-transcription-provider" class="max-w-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="openai">{{ t('admin.ai.transcription.providerOpenAI') }}</SelectItem>
                  <SelectItem value="local">{{ t('admin.ai.transcription.providerLocal') }}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <p
              v-if="transcriptionProvider === 'openai'"
              class="text-xs text-muted-foreground rounded-md border p-3"
            >
              {{ t('admin.ai.transcription.openaiNote') }}
            </p>

            <p
              v-else-if="transcriptionProvider === 'local'"
              class="text-xs text-muted-foreground rounded-md border p-3"
            >
              {{ t('admin.ai.transcription.localNote') }}
            </p>
          </div>

          <Button type="submit" :isLoading="saving">
            {{ t('globals.messages.save') }}
          </Button>
        </form>
        <Spinner v-if="isLoading" />
      </div>
    </template>
    <template #help>
      <p>{{ t('admin.ai.help') }}</p>
    </template>
  </AdminSplitLayout>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AdminSplitLayout from '@/layouts/admin/AdminSplitLayout.vue'
import { Button } from '@shared-ui/components/ui/button'
import { Label } from '@shared-ui/components/ui/label'
import { Switch } from '@shared-ui/components/ui/switch'
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
const transcriptionEnabled = ref(false)
const transcriptionProvider = ref('local')

const showToast = (description, variant) =>
  emitter.emit(EMITTER_EVENTS.SHOW_TOAST, variant ? { variant, description } : { description })

onMounted(async () => {
  try {
    const res = await api.getSettings('ai')
    const data = res.data?.data || {}
    if (data['ai.transcription_enabled'] !== undefined) {
      transcriptionEnabled.value = !!data['ai.transcription_enabled']
    }
    if (data['ai.transcription_provider']) {
      transcriptionProvider.value = data['ai.transcription_provider']
    }
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  } finally {
    isLoading.value = false
  }
})

const onSubmit = async () => {
  saving.value = true
  try {
    await api.updateSettings('ai', {
      'ai.transcription_enabled': !!transcriptionEnabled.value,
      'ai.transcription_provider': transcriptionProvider.value || 'local'
    })
    showToast(t('globals.messages.savedSuccessfully'))
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  } finally {
    saving.value = false
  }
}
</script>
