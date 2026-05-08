<!--
  AISettings — admin page for AI provider configuration + voicemail
  transcription.

  Two unrelated subsystems share this page because they both live behind
  the `ai:manage` permission and the `admin/ai` route:

  - T3b providers: OpenAI / OpenRouter API-key + model + default-flag
    management. Backed by /api/v1/ai/providers and friends.
  - T3v transcription: a tiny key/value form against `setting.ai.*`.

  Each subsystem is a self-contained card so they can be evolved
  independently. The page renders nothing until both finish loading so
  the form values don't pop in mid-render.
-->
<template>
  <AdminSplitLayout>
    <template #content>
      <div :class="{ 'opacity-50 transition-opacity duration-300': isLoading }" class="space-y-6">
        <!-- T3b: OpenAI provider card -->
        <Card>
          <CardHeader>
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <Bot class="h-5 w-5" />
                <CardTitle>{{ t('admin.ai.providers.openai.title') }}</CardTitle>
              </div>
              <div class="flex items-center gap-2">
                <Badge v-if="hasOpenAIKey" class="bg-green-100 text-green-800">
                  <CheckCircle class="h-3 w-3 mr-1" />
                  {{ t('admin.ai.providers.configured') }}
                </Badge>
                <Badge v-else variant="secondary">
                  <AlertCircle class="h-3 w-3 mr-1" />
                  {{ t('admin.ai.providers.notConfigured') }}
                </Badge>
                <Badge v-if="currentDefaultProvider === 'openai'">
                  {{ t('admin.ai.providers.default') }}
                </Badge>
              </div>
            </div>
            <CardDescription>{{ t('admin.ai.providers.openai.description') }}</CardDescription>
          </CardHeader>
          <CardContent class="space-y-4">
            <div class="space-y-2">
              <Label for="openai-key">{{ t('globals.terms.apiKey', 1) }}</Label>
              <Input
                id="openai-key"
                v-model="openaiApiKey"
                type="password"
                :placeholder="hasOpenAIKey ? '********' : 'sk-...'"
              />
              <p class="text-xs text-muted-foreground">
                {{ t('admin.ai.providers.openai.apiKeyHelp') }}
                <a href="https://platform.openai.com/api-keys" target="_blank" class="underline">
                  {{ t('admin.ai.providers.openai.dashboard') }}
                </a>
              </p>
            </div>
            <div class="flex gap-2 flex-wrap">
              <Button @click="saveOpenAI" :disabled="saving || !openaiApiKey">
                {{ t('globals.messages.save') }}
              </Button>
              <Button variant="outline" @click="testProvider('openai')" :disabled="testing">
                <RefreshCw v-if="testing" class="h-4 w-4 mr-2 animate-spin" />
                {{ t('admin.ai.providers.testConnection') }}
              </Button>
              <Button
                v-if="currentDefaultProvider !== 'openai' && hasOpenAIKey"
                variant="secondary"
                @click="setAsDefault('openai')"
              >
                {{ t('admin.ai.providers.setAsDefault') }}
              </Button>
            </div>
          </CardContent>
        </Card>

        <!-- T3b: OpenRouter provider card -->
        <Card>
          <CardHeader>
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <Bot class="h-5 w-5" />
                <CardTitle>{{ t('admin.ai.providers.openrouter.title') }}</CardTitle>
              </div>
              <div class="flex items-center gap-2">
                <Badge v-if="hasOpenRouterKey" class="bg-green-100 text-green-800">
                  <CheckCircle class="h-3 w-3 mr-1" />
                  {{ t('admin.ai.providers.configured') }}
                </Badge>
                <Badge v-else variant="secondary">
                  <AlertCircle class="h-3 w-3 mr-1" />
                  {{ t('admin.ai.providers.notConfigured') }}
                </Badge>
                <Badge v-if="currentDefaultProvider === 'openrouter'">
                  {{ t('admin.ai.providers.default') }}
                </Badge>
              </div>
            </div>
            <CardDescription>{{ t('admin.ai.providers.openrouter.description') }}</CardDescription>
          </CardHeader>
          <CardContent class="space-y-4">
            <div class="space-y-2">
              <Label for="openrouter-key">{{ t('globals.terms.apiKey', 1) }}</Label>
              <Input
                id="openrouter-key"
                v-model="openrouterApiKey"
                type="password"
                :placeholder="hasOpenRouterKey ? '********' : 'sk-or-...'"
              />
              <p class="text-xs text-muted-foreground">
                {{ t('admin.ai.providers.openrouter.apiKeyHelp') }}
                <a href="https://openrouter.ai/keys" target="_blank" class="underline">
                  {{ t('admin.ai.providers.openrouter.dashboard') }}
                </a>
              </p>
            </div>

            <div class="space-y-2">
              <Label for="openrouter-model">{{ t('admin.ai.providers.openrouter.model') }}</Label>
              <Select v-model="openrouterModel">
                <SelectTrigger id="openrouter-model">
                  <SelectValue :placeholder="openrouterModel" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem
                    v-for="model in availableModels"
                    :key="model"
                    :value="model"
                  >
                    {{ model }}
                  </SelectItem>
                </SelectContent>
              </Select>
              <p class="text-xs text-muted-foreground">
                {{ t('admin.ai.providers.openrouter.modelHelp') }}
              </p>
            </div>

            <div class="flex gap-2 flex-wrap">
              <Button @click="saveOpenRouter" :disabled="saving">
                {{ t('globals.messages.save') }}
              </Button>
              <Button
                variant="outline"
                @click="testProvider('openrouter')"
                :disabled="testing || (!hasOpenRouterKey && !openrouterApiKey)"
              >
                <RefreshCw v-if="testing" class="h-4 w-4 mr-2 animate-spin" />
                {{ t('admin.ai.providers.testConnection') }}
              </Button>
              <Button
                v-if="currentDefaultProvider !== 'openrouter' && hasOpenRouterKey"
                variant="secondary"
                @click="setAsDefault('openrouter')"
              >
                {{ t('admin.ai.providers.setAsDefault') }}
              </Button>
            </div>
          </CardContent>
        </Card>

        <!-- T3v: voicemail transcription -->
        <form @submit.prevent="onSubmitTranscription" class="space-y-6 w-full max-w-xl">
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

          <Button type="submit" :isLoading="savingTranscription">
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
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AdminSplitLayout from '@/layouts/admin/AdminSplitLayout.vue'
import { Button } from '@shared-ui/components/ui/button'
import { Label } from '@shared-ui/components/ui/label'
import { Switch } from '@shared-ui/components/ui/switch'
import { Spinner } from '@shared-ui/components/ui/spinner'
import { Input } from '@shared-ui/components/ui/input'
import { Badge } from '@shared-ui/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle
} from '@shared-ui/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@shared-ui/components/ui/select'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import { Bot, CheckCircle, AlertCircle, RefreshCw } from 'lucide-vue-next'
import { useEmitter } from '@/composables/useEmitter'
import { EMITTER_EVENTS } from '@/constants/emitterEvents'
import api from '@/api'

const { t } = useI18n()
const emitter = useEmitter()

const isLoading = ref(true)

// T3b providers state.
const providers = ref([])
const availableModels = ref([])
const saving = ref(false)
const testing = ref(false)
const openaiApiKey = ref('')
const openrouterApiKey = ref('')
// Default model — overwritten by fetchProviders if a value is already saved.
const openrouterModel = ref('anthropic/claude-3-haiku')

const hasOpenAIKey = computed(
  () => providers.value.find((p) => p.provider === 'openai')?.has_api_key || false
)
const hasOpenRouterKey = computed(
  () => providers.value.find((p) => p.provider === 'openrouter')?.has_api_key || false
)
const currentDefaultProvider = computed(
  () => providers.value.find((p) => p.is_default)?.provider || 'openai'
)

// T3v transcription state.
const savingTranscription = ref(false)
const transcriptionEnabled = ref(false)
const transcriptionProvider = ref('local')

const showToast = (description, variant) =>
  emitter.emit(EMITTER_EVENTS.SHOW_TOAST, variant ? { variant, description } : { description })

async function fetchProviders() {
  try {
    const res = await api.getAIProviders()
    providers.value = res.data?.data || []
    const openrouter = providers.value.find((p) => p.provider === 'openrouter')
    if (openrouter?.model) {
      openrouterModel.value = openrouter.model
    }
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  }
}

async function fetchModels() {
  try {
    const res = await api.getAvailableAIModels()
    availableModels.value = res.data?.data || []
  } catch (err) {
    // Non-fatal — the dropdown just won't have options. Admins can
    // still type-and-save through the API directly if needed.
  }
}

async function saveOpenAI() {
  if (!openaiApiKey.value) return
  saving.value = true
  try {
    await api.updateAIProvider({
      provider: 'openai',
      api_key: openaiApiKey.value,
      model: ''
    })
    showToast(t('globals.messages.savedSuccessfully'))
    openaiApiKey.value = ''
    await fetchProviders()
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  } finally {
    saving.value = false
  }
}

async function saveOpenRouter() {
  // Empty key only blocks the very first save — once a key is stored,
  // the admin can change just the model without re-typing.
  if (!openrouterApiKey.value && !hasOpenRouterKey.value) {
    showToast(t('admin.ai.providers.openrouter.apiKeyRequired'), 'destructive')
    return
  }
  saving.value = true
  try {
    await api.updateAIProvider({
      provider: 'openrouter',
      api_key: openrouterApiKey.value || '',
      model: openrouterModel.value
    })
    showToast(t('globals.messages.savedSuccessfully'))
    openrouterApiKey.value = ''
    await fetchProviders()
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  } finally {
    saving.value = false
  }
}

async function setAsDefault(provider) {
  try {
    await api.setDefaultAIProvider({ provider })
    showToast(t('globals.messages.savedSuccessfully'))
    await fetchProviders()
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  }
}

async function testProvider(provider) {
  testing.value = true
  try {
    await api.testAIProvider({
      provider,
      api_key: provider === 'openai' ? openaiApiKey.value : openrouterApiKey.value,
      model: provider === 'openrouter' ? openrouterModel.value : ''
    })
    showToast(t('admin.ai.providers.connectionSuccess'))
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  } finally {
    testing.value = false
  }
}

async function loadTranscriptionSettings() {
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
  }
}

const onSubmitTranscription = async () => {
  savingTranscription.value = true
  try {
    await api.updateSettings('ai', {
      'ai.transcription_enabled': !!transcriptionEnabled.value,
      'ai.transcription_provider': transcriptionProvider.value || 'local'
    })
    showToast(t('globals.messages.savedSuccessfully'))
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  } finally {
    savingTranscription.value = false
  }
}

onMounted(async () => {
  await Promise.all([fetchProviders(), fetchModels(), loadTranscriptionSettings()])
  isLoading.value = false
})
</script>
