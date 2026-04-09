<template>
  <div>
    <Dialog v-model:open="dialogOpen">
      <DialogContent class="max-w-5xl w-full h-[90vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>
            {{
              $t('globals.messages.new', {
                name: $t('globals.terms.conversation').toLowerCase()
              })
            }}
          </DialogTitle>
          <DialogDescription />
        </DialogHeader>
        <form @submit.prevent="createConversation" class="flex flex-col flex-1 overflow-hidden">
          <!-- Form Fields Section -->
          <div class="space-y-4 pb-2 flex-shrink-0">
            <div class="space-y-2">
              <!-- TO field with CC/BCC toggles -->
              <div class="space-y-2">
                <div class="flex items-center space-x-2">
                  <label class="w-10 text-sm font-medium text-muted-foreground shrink-0">TO:</label>
                  <EmailTagInput
                    v-model="emailQuery"
                    :placeholder="t('conversation.searchContact')"
                    class="flex-grow"
                    @blur="handleToBlur"
                    @contactSelected="selectContact"
                  />
                  <div class="flex items-center gap-1 shrink-0">
                    <button
                      v-if="!showCc"
                      type="button"
                      @click="showCc = true"
                      class="text-xs text-muted-foreground hover:text-foreground transition-colors px-1"
                    >Cc</button>
                    <button
                      v-if="!showBcc"
                      type="button"
                      @click="showBcc = true"
                      class="text-xs text-muted-foreground hover:text-foreground transition-colors px-1"
                    >Bcc</button>
                  </div>
                </div>
                <!-- CC field -->
                <div v-if="showCc" class="flex items-center space-x-2">
                  <label class="w-10 text-sm font-medium text-muted-foreground shrink-0">CC:</label>
                  <EmailTagInput
                    v-model="ccEmails"
                    placeholder="Add CC recipients"
                    class="flex-grow"
                  />
                  <button
                    type="button"
                    @click="showCc = false; ccEmails = ''"
                    class="text-muted-foreground hover:text-foreground transition-colors shrink-0 p-1"
                  >
                    <X class="h-3.5 w-3.5" />
                  </button>
                </div>
                <!-- BCC field -->
                <div v-if="showBcc" class="flex items-center space-x-2">
                  <label class="w-10 text-sm font-medium text-muted-foreground shrink-0">BCC:</label>
                  <EmailTagInput
                    v-model="bccEmails"
                    placeholder="Add BCC recipients"
                    class="flex-grow"
                  />
                  <button
                    type="button"
                    @click="showBcc = false; bccEmails = ''"
                    class="text-muted-foreground hover:text-foreground transition-colors shrink-0 p-1"
                  >
                    <X class="h-3.5 w-3.5" />
                  </button>
                </div>
              </div>

              <!-- Name Group -->
              <div class="grid grid-cols-2 gap-4">
                <FormField v-slot="{ componentField }" name="first_name">
                  <FormItem>
                    <FormLabel>{{ $t('globals.terms.firstName') }}</FormLabel>
                    <FormControl>
                      <Input type="text" placeholder="" v-bind="componentField" required />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                </FormField>

                <FormField v-slot="{ componentField }" name="last_name">
                  <FormItem>
                    <FormLabel>{{ $t('globals.terms.lastName') }}</FormLabel>
                    <FormControl>
                      <Input type="text" placeholder="" v-bind="componentField" />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                </FormField>
              </div>

              <!-- Subject and Inbox Group -->
              <div class="grid grid-cols-2 gap-4">
                <FormField v-slot="{ componentField }" name="subject">
                  <FormItem>
                    <FormLabel>{{ $t('globals.terms.subject') }}</FormLabel>
                    <FormControl>
                      <Input type="text" placeholder="" v-bind="componentField" />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                </FormField>

                <FormField v-slot="{ componentField }" name="inbox_id">
                  <FormItem>
                    <FormLabel>{{ $t('globals.terms.inbox') }}</FormLabel>
                    <FormControl>
                      <Select v-bind="componentField">
                        <SelectTrigger>
                          <SelectValue
                            :placeholder="
                              t('globals.messages.select', { name: t('globals.terms.inbox') })
                            "
                          />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectGroup>
                            <SelectItem
                              v-for="option in inboxStore.options"
                              :key="option.value"
                              :value="option.value"
                            >
                              {{ option.label }}
                            </SelectItem>
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                </FormField>
              </div>

              <!-- Assignment Group -->
              <div class="grid grid-cols-2 gap-4">
                <!-- Set assigned team -->
                <FormField v-slot="{ componentField }" name="team_id">
                  <FormItem>
                    <FormLabel>
                      {{
                        $t('globals.messages.assign', {
                          name: t('globals.terms.team').toLowerCase()
                        })
                      }}
                      ({{ $t('globals.terms.optional').toLowerCase() }})
                    </FormLabel>
                    <FormControl>
                      <SelectComboBox
                        v-bind="componentField"
                        :items="[{ value: 'none', label: 'None' }, ...teamStore.options]"
                        :placeholder="
                          t('globals.messages.select', { name: t('globals.terms.team') })
                        "
                        type="team"
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                </FormField>

                <!-- Set assigned agent -->
                <FormField v-slot="{ componentField }" name="agent_id">
                  <FormItem>
                    <FormLabel>
                      {{
                        $t('globals.messages.assign', {
                          name: t('globals.terms.agent').toLowerCase()
                        })
                      }}
                      ({{ $t('globals.terms.optional').toLowerCase() }})
                    </FormLabel>
                    <FormControl>
                      <SelectComboBox
                        v-bind="componentField"
                        :items="[{ value: 'none', label: 'None' }, ...uStore.options]"
                        :placeholder="
                          t('globals.messages.select', { name: t('globals.terms.agent') })
                        "
                        type="user"
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                </FormField>
              </div>
            </div>
          </div>

          <!-- Message Editor Section -->
          <div class="flex-1 flex flex-col min-h-0 mt-4">
            <FormField v-slot="{ componentField }" name="content">
              <FormItem class="flex flex-col h-full">
                <FormLabel>{{ $t('globals.terms.message') }}</FormLabel>
                <FormControl class="flex-1 flex flex-col min-h-0">
                  <div class="flex flex-col h-full">
                    <Editor
                      ref="createEditorRef"
                      v-model:htmlContent="componentField.modelValue"
                      @update:htmlContent="(value) => componentField.onChange(value)"
                      :placeholder="t('editor.newLine') + t('editor.ctrlK')"
                      :insertContent="insertContent"
                      :autoFocus="false"
                      class="w-full flex-1 overflow-y-auto p-2 box min-h-0"
                      @send="createConversation"
                    />

                    <!-- Macro preview -->
                    <MacroActionsPreview
                      v-if="conversationStore.getMacro(MACRO_CONTEXT.NEW_CONVERSATION).actions?.length > 0"
                      :actions="conversationStore.getMacro(MACRO_CONTEXT.NEW_CONVERSATION)?.actions || []"
                      :onRemove="
                        (action) => conversationStore.removeMacroAction(action, MACRO_CONTEXT.NEW_CONVERSATION)
                      "
                      class="mt-2 flex-shrink-0"
                    />

                    <!-- Attachments preview -->
                    <AttachmentsPreview
                      :attachments="mediaFiles"
                      :uploadingFiles="uploadingFiles"
                      :onDelete="handleFileDelete"
                      v-if="mediaFiles.length > 0 || uploadingFiles.length > 0"
                      class="mt-2 flex-shrink-0"
                    />
                  </div>
                </FormControl>
                <FormMessage />
              </FormItem>
            </FormField>
          </div>

          <DialogFooter class="mt-4 pt-2 flex items-center !justify-between w-full flex-shrink-0">
            <ReplyBoxMenuBar
              :handleFileUpload="handleFileUpload"
              @emojiSelect="handleEmojiSelect"
              @editorCommand="(cmd) => createEditorRef?.runCommand(cmd)"
              @generateResponse="handleGenerateResponse"
              :showSendButton="false"
              :isGenerating="isGenerating"
            />
            <Button type="submit" :disabled="isDisabled" :isLoading="loading">
              {{ $t('globals.messages.submit') }}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup>
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'
import { FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { z } from 'zod'
import { ref, watch, onUnmounted, nextTick, onMounted, computed } from 'vue'
import AttachmentsPreview from '@/features/conversation/message/attachment/AttachmentsPreview.vue'
import { useConversationStore } from '@/stores/conversation'
import MacroActionsPreview from '@/features/conversation/MacroActionsPreview.vue'
import ReplyBoxMenuBar from '@/features/conversation/ReplyBoxMenuBar.vue'
import { EMITTER_EVENTS } from '@/constants/emitterEvents.js'
import { MACRO_CONTEXT } from '@/constants/conversation'
import { useEmitter } from '@/composables/useEmitter'
import { handleHTTPError } from '@/utils/http'
import { useInboxStore } from '@/stores/inbox'
import { useUsersStore } from '@/stores/users'
import { useTeamStore } from '@/stores/team'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { useI18n } from 'vue-i18n'
import { useFileUpload } from '@/composables/useFileUpload'
import Editor from '@/components/editor/TextEditor.vue'
import { useMacroStore } from '@/stores/macro'
import SelectComboBox from '@/components/combobox/SelectCombobox.vue'
import { UserTypeAgent } from '@/constants/user'
import api from '@/api'
import EmailTagInput from '@/components/EmailTagInput.vue'
import { X } from 'lucide-vue-next'
import { useUserStore } from '@/stores/user'

const dialogOpen = defineModel({
  required: false,
  default: () => false
})

const inboxStore = useInboxStore()
const { t } = useI18n()
const uStore = useUsersStore()
const teamStore = useTeamStore()
const userStore = useUserStore()
const emitter = useEmitter()
const loading = ref(false)
const emailQuery = ref('')
const conversationStore = useConversationStore()
const macroStore = useMacroStore()
const insertContent = ref('')
const createEditorRef = ref(null)
const showCc = ref(false)
const showBcc = ref(false)
const ccEmails = ref('')
const bccEmails = ref('')
const currentSignature = ref('')

const handleEmojiSelect = (emoji) => {
  insertContent.value = undefined
  // Force reactivity so the user can select the same emoji multiple times
  nextTick(() => (insertContent.value = emoji))
}

const { uploadingFiles, handleFileUpload, handleFileDelete, mediaFiles, clearMediaFiles } =
  useFileUpload({
    linkedModel: 'messages'
  })

const isDisabled = computed(() => {
  if (loading.value || uploadingFiles.value.length > 0) {
    return true
  }
  return false
})

const formSchema = z.object({
  subject: z.string().min(
    1,
    t('globals.messages.cannotBeEmpty', {
      name: t('globals.terms.subject')
    })
  ),
  content: z.string().min(
    1,
    t('globals.messages.cannotBeEmpty', {
      name: t('globals.terms.message')
    })
  ),
  inbox_id: z.any().refine((val) => inboxStore.options.some((option) => option.value === val), {
    message: t('globals.messages.required')
  }),
  team_id: z.any().optional(),
  agent_id: z.any().optional(),
  contact_email: z.string().email(t('globals.messages.invalidEmailAddress')),
  first_name: z.string().min(1, t('globals.messages.required')),
  last_name: z.string().optional()
})

onUnmounted(() => {
  clearMediaFiles()
  conversationStore.resetMacro(MACRO_CONTEXT.NEW_CONVERSATION)
  conversationStore.isNewConversationOpen = false
  emitter.emit(EMITTER_EVENTS.SET_NESTED_COMMAND, {
    command: null,
    open: false
  })
})

onMounted(() => {
  macroStore.setCurrentView('starting_conversation')
  conversationStore.isNewConversationOpen = true
  emitter.emit(EMITTER_EVENTS.SET_NESTED_COMMAND, {
    command: 'apply-macro-to-new-conversation',
    open: false
  })
})

const form = useForm({
  validationSchema: toTypedSchema(formSchema),
  initialValues: {
    inbox_id: null,
    team_id: null,
    agent_id: null,
    subject: '',
    content: '',
    contact_email: '',
    first_name: '',
    last_name: ''
  }
})

watch(emailQuery, (newVal) => {
  const firstEmail = newVal.split(',').map(e => e.trim()).filter(e => e)[0] || ''
  form.setFieldValue('contact_email', firstEmail)
})

/**
 * Fetch and insert signature when inbox changes.
 */
const fetchAndInsertSignature = async (inboxId) => {
  if (!inboxId) return
  try {
    const resp = await api.getInboxSignature(inboxId, '')
    const signature = resp.data?.data?.signature || ''
    currentSignature.value = signature

    const currentContent = form.values.content || ''
    const sigBlock = signature
      ? '<div class="email-signature">' + signature + '</div>'
      : ''

    // Replace existing signature or set as initial content
    if (currentContent.includes('class="email-signature"')) {
      const newContent = sigBlock
        ? currentContent.replace(/<div class="email-signature">[\s\S]*?<\/div>/, sigBlock)
        : currentContent.replace(/<p><br><\/p><div class="email-signature">[\s\S]*?<\/div>/, '')
      form.setFieldValue('content', newContent)
    } else if (signature) {
      // If content is empty or just whitespace, set signature as content
      const stripped = currentContent.replace(/<[^>]*>/g, '').trim()
      if (!stripped) {
        form.setFieldValue('content', '<p><br></p>' + sigBlock)
      } else {
        form.setFieldValue('content', currentContent + '<p><br></p>' + sigBlock)
      }
    }
  } catch (err) {
    currentSignature.value = ''
  }
}

// Watch inbox_id changes to fetch signature
watch(
  () => form.values.inbox_id,
  (newInboxId) => {
    if (newInboxId) {
      fetchAndInsertSignature(Number(newInboxId))
    }
  }
)

// Auto-select inbox: prefer agent's team default inbox, fallback to first
watch(
  () => inboxStore.options,
  async (options) => {
    if (options.length > 0 && !form.values.inbox_id) {
      let selectedInbox = options[0].value

      // Try to find agent's team default inbox and auto-set team
      const agentTeams = userStore.teams
      if (agentTeams.length > 0) {
        await teamStore.fetchTeams()
        for (const agentTeam of agentTeams) {
          const team = teamStore.teams.find(t => t.id === agentTeam.id)
          if (team?.default_inbox_id) {
            const matchingOption = options.find(o => Number(o.value) === team.default_inbox_id)
            if (matchingOption) {
              selectedInbox = matchingOption.value
              if (!form.values.team_id) {
                form.setFieldValue("team_id", String(team.id))
              }
              break
            }
          }
        }
      }

      form.setFieldValue("inbox_id", selectedInbox)
      fetchAndInsertSignature(Number(selectedInbox))
    }

    // Auto-assign current agent
    if (!form.values.agent_id && userStore.userID) {
      form.setFieldValue("agent_id", String(userStore.userID))
    }
  },
  { immediate: true }
)

// When team is selected, auto-set the inbox to team's default
watch(
  () => form.values.team_id,
  async (newTeamId) => {
    if (!newTeamId || newTeamId === 'none') return
    await teamStore.fetchTeams()
    const team = teamStore.teams.find(t => t.id === Number(newTeamId))
    if (team?.default_inbox_id) {
      form.setFieldValue('inbox_id', String(team.default_inbox_id))
    }
  }
)



const handleToBlur = () => {
  // Sync first email to form validation field
  const firstEmail = emailQuery.value.split(',').map(e => e.trim()).filter(e => e)[0] || ''
  form.setFieldValue('contact_email', firstEmail)
}

const selectContact = (contact) => {
  // When EmailTagInput emits contactSelected, add email and fill name fields
  const current = emailQuery.value ? emailQuery.value.split(',').map(e => e.trim()).filter(e => e) : []
  if (!current.includes(contact.email)) {
    current.push(contact.email)
  }
  emailQuery.value = current.join(', ')
  // Only fill name if first/last are empty (don't overwrite for additional recipients)
  if (!form.values.first_name) {
    form.setFieldValue('first_name', contact.first_name || '')
  }
  if (!form.values.last_name) {
    form.setFieldValue('last_name', contact.last_name || '')
  }
  form.setFieldValue('contact_email', current[0])
}

const isGenerating = ref(false)
const handleGenerateResponse = async () => {
  isGenerating.value = true
  try {
    // Use editor content as context for generation
    const editorHtml = form.values.content || ''
    const doc = new DOMParser().parseFromString(editorHtml, 'text/html')
    const editorText = (doc.body.textContent || '').trim()

    if (!editorText) {
      emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
        variant: 'destructive',
        description: 'Type some context first (e.g. customer question), then click Generate'
      })
      return
    }

    const resp = await api.ragGenerate({
      conversation_id: 0,
      inbox_id: Number(form.values.inbox_id) || 0,
      customer_message: `Customer inquiry: ${editorText}`,
      include_ecommerce: false,
      agent_instructions: ''
    })

    if (resp.data?.data?.response) {
      const response = resp.data.data.response
      let generatedHtml
      if (/<[a-z][\s\S]*>/i.test(response)) {
        generatedHtml = response
          .replace(/>\s*\n\s*/g, '>')
          .replace(/\s*\n\s*</g, '<')
          .replace(/\n/g, '<br>')
          .replace(/<li>\s*(<br\s*\/?>\s*)*<\/li>/gi, '')
          .replace(/<p>\s*(<br\s*\/?>\s*)*<\/p>/gi, '')
      } else {
        generatedHtml = '<p>' + response.replace(/\n{2,}/g, '</p><p>').replace(/\n/g, '<br>') + '</p>'
      }
      generatedHtml = generatedHtml.replace(/<script[\s\S]*?<\/script>/gi, '')
      generatedHtml = generatedHtml.replace(/\son\w+\s*=\s*("[^"]*"|'[^']*'|[^\s>]*)/gi, '')

      // Set content via the editor ref (clears existing and replaces)
      createEditorRef.value?.editor?.commands.setContent(generatedHtml)
    }
  } catch (err) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: 'Failed to generate response'
    })
  } finally {
    isGenerating.value = false
  }
}

const createConversation = form.handleSubmit(async (values) => {
  loading.value = true
  try {
    // Convert ids to numbers if they are not already
    values.inbox_id = Number(values.inbox_id)
    values.team_id = values.team_id ? Number(values.team_id) : null
    values.agent_id = values.agent_id ? Number(values.agent_id) : null
    // Array of attachment ids.
    values.attachments = mediaFiles.value.map((file) => file.id)
    values.cc = ccEmails.value || ''
    values.bcc = bccEmails.value || ''
    // Initiator of this conversation is always agent
    values.initiator = UserTypeAgent
    const conversation = await api.createConversation(values)
    const conversationUUID = conversation.data.data.uuid

    // Get macro from context, and set if any actions are available.
    const macro = conversationStore.getMacro(MACRO_CONTEXT.NEW_CONVERSATION)
    if (conversationUUID !== '' && macro?.id && macro?.actions?.length > 0) {
      try {
        await api.applyMacro(conversationUUID, macro.id, macro.actions)
      } catch (error) {
        emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
          variant: 'destructive',
          description: handleHTTPError(error).message
        })
      }
    }
    dialogOpen.value = false
    form.resetForm()
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  } finally {
    loading.value = false
  }
})

/**
 * Watches for changes in the macro id and update message content.
 */
watch(
  () => conversationStore.getMacro(MACRO_CONTEXT.NEW_CONVERSATION).id,
  (newId) => {
    if (!newId) return
    const macroContent = conversationStore.getMacro(MACRO_CONTEXT.NEW_CONVERSATION).message_content
    if (!macroContent) return
    // Replace placeholders with form values and insert
    const replaced = macroContent
      .replaceAll('{{contact.first_name}}', form.values.first_name || '')
      .replaceAll('{{contact.last_name}}', form.values.last_name || '')
      .replaceAll('{{contact.full_name}}', [form.values.first_name, form.values.last_name].filter(Boolean).join(' ') || '')
      .replaceAll('{{contact.email}}', form.values.contact_email || '')
    insertContent.value = undefined
    nextTick(() => {
      insertContent.value = replaced
    })
  },
  { deep: true }
)
</script>
