<template>
  <AdminSplitLayout>
    <template #content>
      <div :class="{ 'opacity-50 transition-opacity duration-300': isLoading }" class="space-y-6">
        <form @submit.prevent="onSubmit" class="space-y-6 w-full max-w-xl">
          <div class="space-y-2">
            <Label for="auto-trash-resolved">{{ t('admin.trash.autoTrashResolvedDays') }}</Label>
            <Input
              id="auto-trash-resolved"
              v-model.number="autoTrashResolvedDays"
              type="number"
              min="0"
              placeholder="90"
            />
            <p class="text-xs text-muted-foreground">{{ t('admin.trash.autoTrashResolvedDays.description') }}</p>
          </div>

          <div class="space-y-2">
            <Label for="auto-trash-spam">{{ t('admin.trash.autoTrashSpamDays') }}</Label>
            <Input
              id="auto-trash-spam"
              v-model.number="autoTrashSpamDays"
              type="number"
              min="0"
              placeholder="30"
            />
            <p class="text-xs text-muted-foreground">{{ t('admin.trash.autoTrashSpamDays.description') }}</p>
          </div>

          <div class="space-y-2">
            <Label for="auto-delete">{{ t('admin.trash.autoDeleteDays') }}</Label>
            <Input
              id="auto-delete"
              v-model.number="autoDeleteDays"
              type="number"
              min="0"
              placeholder="30"
            />
            <p class="text-xs text-muted-foreground">{{ t('admin.trash.autoDeleteDays.description') }}</p>
          </div>

          <div class="space-y-2">
            <Label for="activity-purge">{{ t('admin.trash.activityPurgeDays') }}</Label>
            <Input
              id="activity-purge"
              v-model.number="activityPurgeDays"
              type="number"
              min="0"
              placeholder="7"
            />
            <p class="text-xs text-muted-foreground">{{ t('admin.trash.activityPurgeDays.description') }}</p>
          </div>

          <Button type="submit" :isLoading="saving">
            {{ t('globals.messages.save') }}
          </Button>
        </form>
        <Spinner v-if="isLoading" />
      </div>
    </template>
    <template #help>
      <p>{{ t('admin.trash.help') }}</p>
    </template>
  </AdminSplitLayout>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AdminSplitLayout from '@/layouts/admin/AdminSplitLayout.vue'
import { Button } from '@shared-ui/components/ui/button'
import { Input } from '@shared-ui/components/ui/input'
import { Label } from '@shared-ui/components/ui/label'
import { Spinner } from '@shared-ui/components/ui/spinner'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import { useEmitter } from '@/composables/useEmitter'
import { EMITTER_EVENTS } from '@/constants/emitterEvents'
import api from '@/api'

const { t } = useI18n()
const emitter = useEmitter()

const isLoading = ref(true)
const saving = ref(false)
const autoTrashResolvedDays = ref(90)
const autoTrashSpamDays = ref(30)
const autoDeleteDays = ref(30)
const activityPurgeDays = ref(7)

const showToast = (description, variant) =>
  emitter.emit(EMITTER_EVENTS.SHOW_TOAST, variant ? { variant, description } : { description })

onMounted(async () => {
  try {
    const res = await api.getSettings('trash')
    const data = res.data?.data || {}
    if (data['trash.auto_trash_resolved_days'] !== undefined) autoTrashResolvedDays.value = data['trash.auto_trash_resolved_days']
    if (data['trash.auto_trash_spam_days'] !== undefined) autoTrashSpamDays.value = data['trash.auto_trash_spam_days']
    if (data['trash.auto_delete_days'] !== undefined) autoDeleteDays.value = data['trash.auto_delete_days']
    if (data['trash.activity_purge_days'] !== undefined) activityPurgeDays.value = data['trash.activity_purge_days']
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  } finally {
    isLoading.value = false
  }
})

const onSubmit = async () => {
  saving.value = true
  try {
    await api.updateSettings('trash', {
      'trash.auto_trash_resolved_days': Number(autoTrashResolvedDays.value) || 0,
      'trash.auto_trash_spam_days': Number(autoTrashSpamDays.value) || 0,
      'trash.auto_delete_days': Number(autoDeleteDays.value) || 0,
      'trash.activity_purge_days': Number(activityPurgeDays.value) || 0
    })
    showToast(t('globals.messages.savedSuccessfully'))
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  } finally {
    saving.value = false
  }
}
</script>
