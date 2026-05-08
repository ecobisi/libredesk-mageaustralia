<!--
  T3a Knowledge Sources admin UI.

  Three responsibilities:
    1. Test Knowledge Base — type a query, see what rag_documents
       come back at the configured threshold. Useful for tuning the
       knowledge base before flipping the Generate Response button on
       for agents.
    2. Source CRUD — list / add / edit / sync / delete macros, web
       pages, and uploaded files (txt / csv / json).
    3. File upload — separate dialog because it goes through a
       multipart endpoint while macro/webpage sources go through JSON.

  Adapted from v1.0.3's AdminPageWithHelp layout to v2's
  AdminSplitLayout, and from raw English strings to i18n keys.
-->
<template>
  <AdminSplitLayout>
    <template #content>
      <div class="space-y-6">
        <!-- Test Query Card -->
        <Card>
          <CardHeader>
            <div class="flex items-center gap-2">
              <Search class="h-5 w-5" />
              <CardTitle>{{ t('admin.knowledgeSources.testTitle') }}</CardTitle>
            </div>
            <CardDescription>{{ t('admin.knowledgeSources.testDescription') }}</CardDescription>
          </CardHeader>
          <CardContent class="space-y-4">
            <div class="flex gap-2">
              <Input
                v-model="testQuery"
                :placeholder="t('admin.knowledgeSources.testPlaceholder')"
                class="flex-1"
                @keyup.enter="runTestQuery"
              />
              <Button @click="runTestQuery" :disabled="testLoading">
                <Search v-if="!testLoading" class="w-4 h-4 mr-2" />
                <RefreshCw v-else class="w-4 h-4 mr-2 animate-spin" />
                {{ t('globals.messages.search') }}
              </Button>
            </div>

            <div v-if="testResults.length > 0" class="space-y-3">
              <Label>{{ t('admin.knowledgeSources.resultsCount', { count: testResults.length }) }}</Label>
              <div class="space-y-2 max-h-[400px] overflow-y-auto">
                <div
                  v-for="(result, index) in testResults"
                  :key="index"
                  class="p-3 border rounded-lg bg-muted/50"
                >
                  <div class="flex items-center justify-between mb-2">
                    <Badge variant="outline">{{ result.source_ref || 'document' }}</Badge>
                    <Badge class="bg-green-100 text-green-800">
                      {{ formatScore(result.similarity) }}
                    </Badge>
                  </div>
                  <p class="font-medium text-sm mb-1">{{ result.title }}</p>
                  <p class="text-sm whitespace-pre-wrap text-muted-foreground">
                    {{ result.content?.substring(0, 500) }}{{ result.content?.length > 500 ? '...' : '' }}
                  </p>
                </div>
              </div>
            </div>

            <div v-else-if="testQuery && !testLoading" class="text-sm text-muted-foreground text-center py-4">
              {{ t('admin.knowledgeSources.noResults') }}
            </div>
          </CardContent>
        </Card>

        <!-- Knowledge Sources -->
        <div class="flex items-center justify-between">
          <div>
            <h2 class="text-lg font-semibold">{{ t('admin.knowledgeSources.title') }}</h2>
            <p class="text-sm text-muted-foreground">{{ t('admin.knowledgeSources.description') }}</p>
          </div>
          <div class="flex gap-2">
            <Button variant="outline" @click="openUploadDialog">
              <Upload class="w-4 h-4 mr-2" />
              {{ t('admin.knowledgeSources.uploadFile') }}
            </Button>
            <Button @click="openAddDialog">
              <Plus class="w-4 h-4 mr-2" />
              {{ t('admin.knowledgeSources.addSource') }}
            </Button>
          </div>
        </div>

        <div v-if="loading" class="flex justify-center py-8">
          <Spinner />
        </div>

        <div v-else-if="sources.length === 0" class="text-center py-8 text-muted-foreground">
          <Database class="w-12 h-12 mx-auto mb-4 opacity-50" />
          <p>{{ t('admin.knowledgeSources.empty') }}</p>
          <p class="text-sm">{{ t('admin.knowledgeSources.emptyHint') }}</p>
        </div>

        <div v-else class="grid gap-4">
          <Card v-for="source in sources" :key="source.id">
            <CardHeader class="pb-3">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-3">
                  <component :is="getSourceIcon(source.source_type)" class="w-5 h-5 text-muted-foreground" />
                  <div>
                    <CardTitle class="text-base">{{ source.name }}</CardTitle>
                    <CardDescription class="flex items-center gap-2 mt-1">
                      <Badge variant="outline">{{ source.source_type }}</Badge>
                      <Badge v-if="source.enabled" class="bg-green-500">
                        {{ t('globals.messages.enabled') }}
                      </Badge>
                      <Badge v-else variant="secondary">
                        {{ t('globals.messages.disabled') }}
                      </Badge>
                    </CardDescription>
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <Button
                    v-if="source.source_type !== 'file'"
                    variant="outline"
                    size="sm"
                    @click="syncSource(source)"
                    :disabled="syncing[source.id]"
                  >
                    <RefreshCw :class="['w-4 h-4 mr-1', syncing[source.id] ? 'animate-spin' : '']" />
                    {{ t('admin.knowledgeSources.sync') }}
                  </Button>
                  <Button
                    v-if="source.source_type !== 'file'"
                    variant="outline"
                    size="sm"
                    @click="openEditDialog(source)"
                  >
                    {{ t('globals.messages.edit') }}
                  </Button>
                  <Button variant="destructive" size="sm" @click="deleteSource(source)">
                    <Trash2 class="w-4 h-4" />
                  </Button>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <div class="flex items-center gap-4 text-sm text-muted-foreground">
                <div class="flex items-center gap-1">
                  <Clock class="w-4 h-4" />
                  {{ t('admin.knowledgeSources.lastSynced') }}: {{ formatDate(source.last_synced_at) }}
                </div>
                <div v-if="source.source_type === 'file' && source.config?.filename" class="flex items-center gap-1">
                  <FileText class="w-4 h-4" />
                  {{ source.config.filename }}
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>

      <!-- Add/Edit Dialog -->
      <Dialog :open="showAddDialog" @update:open="showAddDialog = $event">
        <DialogContent class="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>
              {{ editingSource ? t('admin.knowledgeSources.editTitle') : t('admin.knowledgeSources.addTitle') }}
            </DialogTitle>
            <DialogDescription>{{ t('admin.knowledgeSources.dialogDescription') }}</DialogDescription>
          </DialogHeader>

          <div class="space-y-4 py-4">
            <div class="space-y-2">
              <Label>{{ t('globals.terms.name') }}</Label>
              <Input v-model="formData.name" :placeholder="t('admin.knowledgeSources.namePlaceholder')" />
            </div>

            <div class="space-y-2">
              <Label>{{ t('globals.terms.type') }}</Label>
              <Select v-model="formData.source_type" :disabled="!!editingSource">
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="macro">{{ t('admin.knowledgeSources.typeMacro') }}</SelectItem>
                  <SelectItem value="webpage">{{ t('admin.knowledgeSources.typeWebpage') }}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div v-if="formData.source_type === 'webpage'" class="space-y-2">
              <Label>{{ t('admin.knowledgeSources.urlsLabel') }}</Label>
              <Textarea
                v-model="formData.urls"
                :placeholder="t('admin.knowledgeSources.urlsPlaceholder')"
                rows="4"
              />
            </div>

            <div class="flex items-center gap-2">
              <Switch v-model:checked="formData.enabled" />
              <Label>{{ t('globals.messages.enabled') }}</Label>
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" @click="showAddDialog = false">
              {{ t('globals.messages.cancel') }}
            </Button>
            <Button @click="saveSource" :disabled="saving">
              <Spinner v-if="saving" class="w-4 h-4 mr-2" />
              {{ editingSource ? t('globals.messages.save') : t('globals.messages.create') }}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <!-- File Upload Dialog -->
      <Dialog :open="showUploadDialog" @update:open="showUploadDialog = $event">
        <DialogContent class="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{{ t('admin.knowledgeSources.uploadTitle') }}</DialogTitle>
            <DialogDescription>{{ t('admin.knowledgeSources.uploadDescription') }}</DialogDescription>
          </DialogHeader>

          <div class="space-y-4 py-4">
            <div class="space-y-2">
              <Label>{{ t('admin.knowledgeSources.fileLabel') }}</Label>
              <input
                ref="fileInputRef"
                type="file"
                accept=".txt,.csv,.json"
                class="hidden"
                @change="handleFileSelect"
              />
              <div
                class="border-2 border-dashed rounded-lg p-6 text-center cursor-pointer hover:border-primary transition-colors"
                @click="triggerFileInput"
              >
                <Upload class="w-8 h-8 mx-auto mb-2 text-muted-foreground" />
                <p v-if="!uploadFile" class="text-sm text-muted-foreground">
                  {{ t('admin.knowledgeSources.uploadHint') }}
                </p>
                <p v-else class="text-sm font-medium">{{ uploadFile.name }}</p>
                <p class="text-xs text-muted-foreground mt-1">
                  {{ t('admin.knowledgeSources.uploadSupported') }}
                </p>
              </div>
            </div>

            <div class="space-y-2">
              <Label>{{ t('admin.knowledgeSources.uploadNameOptional') }}</Label>
              <Input v-model="uploadName" :placeholder="t('admin.knowledgeSources.uploadNamePlaceholder')" />
              <p class="text-xs text-muted-foreground">{{ t('admin.knowledgeSources.uploadNameHint') }}</p>
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" @click="showUploadDialog = false">
              {{ t('globals.messages.cancel') }}
            </Button>
            <Button @click="uploadFileSource" :disabled="uploading || !uploadFile">
              <Spinner v-if="uploading" class="w-4 h-4 mr-2" />
              <Upload v-else class="w-4 h-4 mr-2" />
              {{ t('admin.knowledgeSources.upload') }}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </template>

    <template #help>
      <p>{{ t('admin.knowledgeSources.help') }}</p>
    </template>
  </AdminSplitLayout>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AdminSplitLayout from '@/layouts/admin/AdminSplitLayout.vue'
import api from '@/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@shared-ui/components/ui/card'
import { Button } from '@shared-ui/components/ui/button'
import { Input } from '@shared-ui/components/ui/input'
import { Label } from '@shared-ui/components/ui/label'
import { Switch } from '@shared-ui/components/ui/switch'
import { Textarea } from '@shared-ui/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@shared-ui/components/ui/select'
import { Badge } from '@shared-ui/components/ui/badge'
import { Spinner } from '@shared-ui/components/ui/spinner'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@shared-ui/components/ui/dialog'
import {
  Database,
  RefreshCw,
  Plus,
  Trash2,
  Globe,
  MessageSquare,
  FileText,
  Clock,
  Search,
  Upload
} from 'lucide-vue-next'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import { useEmitter } from '@/composables/useEmitter'
import { EMITTER_EVENTS } from '@/constants/emitterEvents'

const { t } = useI18n()
const emitter = useEmitter()

const showToast = (description, variant) =>
  emitter.emit(EMITTER_EVENTS.SHOW_TOAST, variant ? { variant, description } : { description })

const sources = ref([])
const loading = ref(true)
const syncing = ref({})

const testQuery = ref('')
const testResults = ref([])
const testLoading = ref(false)

const showAddDialog = ref(false)
const editingSource = ref(null)
const formData = ref({
  name: '',
  source_type: 'macro',
  enabled: true,
  urls: ''
})
const saving = ref(false)

const showUploadDialog = ref(false)
const uploadFile = ref(null)
const uploadName = ref('')
const uploading = ref(false)
const fileInputRef = ref(null)

async function fetchSources() {
  loading.value = true
  try {
    const res = await api.getRAGSources()
    sources.value = res.data.data || []
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  } finally {
    loading.value = false
  }
}

async function runTestQuery() {
  if (!testQuery.value.trim()) {
    showToast(t('admin.knowledgeSources.queryRequired'), 'destructive')
    return
  }
  testLoading.value = true
  testResults.value = []
  try {
    const res = await api.ragSearch({
      query: testQuery.value,
      limit: 5,
      threshold: 0.25
    })
    testResults.value = res.data.data || []
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  } finally {
    testLoading.value = false
  }
}

function openAddDialog() {
  editingSource.value = null
  formData.value = { name: '', source_type: 'macro', enabled: true, urls: '' }
  showAddDialog.value = true
}

function openEditDialog(source) {
  editingSource.value = source
  formData.value = {
    name: source.name,
    source_type: source.source_type,
    enabled: source.enabled,
    urls: source.config?.urls?.join('\n') || ''
  }
  showAddDialog.value = true
}

async function saveSource() {
  if (!formData.value.name.trim()) {
    showToast(t('admin.knowledgeSources.nameRequired'), 'destructive')
    return
  }
  saving.value = true
  try {
    const config = {}
    if (formData.value.source_type === 'webpage') {
      config.urls = formData.value.urls.split('\n').filter((u) => u.trim())
    }
    const data = {
      name: formData.value.name,
      source_type: formData.value.source_type,
      enabled: formData.value.enabled,
      config
    }
    if (editingSource.value) {
      await api.updateRAGSource(editingSource.value.id, data)
    } else {
      await api.createRAGSource(data)
    }
    showToast(t('globals.messages.savedSuccessfully'))
    showAddDialog.value = false
    await fetchSources()
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  } finally {
    saving.value = false
  }
}

async function deleteSource(source) {
  if (!window.confirm(t('admin.knowledgeSources.deleteConfirm', { name: source.name }))) return
  try {
    await api.deleteRAGSource(source.id)
    showToast(t('globals.messages.deletedSuccessfully', { name: source.name }))
    await fetchSources()
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  }
}

async function syncSource(source) {
  syncing.value[source.id] = true
  try {
    await api.syncRAGSource(source.id)
    showToast(t('admin.knowledgeSources.syncStarted'))
    // Server sync runs in the background; refresh the list after a
    // short delay to pick up the new last_synced_at.
    setTimeout(() => fetchSources(), 3000)
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  } finally {
    syncing.value[source.id] = false
  }
}

function getSourceIcon(type) {
  switch (type) {
    case 'macro':
      return MessageSquare
    case 'webpage':
      return Globe
    case 'file':
      return FileText
    default:
      return FileText
  }
}

function formatDate(date) {
  if (!date) return t('admin.knowledgeSources.never')
  return new Date(date).toLocaleString()
}

function formatScore(score) {
  return (score * 100).toFixed(1) + '%'
}

function openUploadDialog() {
  uploadFile.value = null
  uploadName.value = ''
  showUploadDialog.value = true
}

function handleFileSelect(event) {
  const file = event.target.files?.[0]
  if (file) {
    uploadFile.value = file
    uploadName.value = file.name.replace(/\.[^/.]+$/, '')
  }
}

function triggerFileInput() {
  fileInputRef.value?.click()
}

async function uploadFileSource() {
  if (!uploadFile.value) {
    showToast(t('admin.knowledgeSources.fileRequired'), 'destructive')
    return
  }
  const validTypes = ['.txt', '.csv', '.json']
  const ext = '.' + uploadFile.value.name.split('.').pop().toLowerCase()
  if (!validTypes.includes(ext)) {
    showToast(t('admin.knowledgeSources.unsupportedType'), 'destructive')
    return
  }
  uploading.value = true
  try {
    const formData = new FormData()
    formData.append('file', uploadFile.value)
    if (uploadName.value.trim()) {
      formData.append('name', uploadName.value.trim())
    }
    formData.append('enabled', 'true')

    await api.ragFileUpload(formData)
    showToast(t('admin.knowledgeSources.uploadSuccess'))
    showUploadDialog.value = false
    await fetchSources()
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  } finally {
    uploading.value = false
  }
}

onMounted(() => {
  fetchSources()
})
</script>
