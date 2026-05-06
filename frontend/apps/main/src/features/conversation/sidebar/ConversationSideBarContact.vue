<template>
  <div class="space-y-2">
    <div class="flex justify-between items-start">
      <div class="relative">
        <Avatar class="size-20">
          <AvatarImage
            :src="conversation?.contact?.avatar_url || ''"
          />
          <AvatarFallback>
            {{ conversation?.contact?.first_name?.toUpperCase().substring(0, 2) }}
          </AvatarFallback>
        </Avatar>
        <StatusDot
          v-if="isLivechat"
          :status="contactStatus"
          size="lg"
          class="absolute bottom-1 right-1 border-2 border-background"
        />
      </div>
      <Button
        variant="ghost"
        size="icon"
        class="h-7 w-7"
        @click="emitter.emit(EMITTER_EVENTS.CONVERSATION_SIDEBAR_TOGGLE)"
      >
        <ViewVerticalIcon />
      </Button>
    </div>

    <div class="flex items-center gap-2 group/contact">
      <span v-if="conversationStore.conversation.loading">
        <Skeleton class="w-24 h-4" />
      </span>
      <router-link
        v-else-if="userStore.can('contacts:read') && conversation?.contact_id"
        :to="{ name: 'contact-detail', params: { id: conversation.contact_id } }"
        class="flex items-center gap-2 hover:underline cursor-pointer"
      >
        {{ conversation?.contact?.first_name + ' ' + conversation?.contact?.last_name }}
        <ExternalLink size="16" class="text-muted-foreground flex-shrink-0" />
      </router-link>
      <span v-else>
        {{ conversation?.contact?.first_name + ' ' + conversation?.contact?.last_name }}
      </span>
      <!-- Change-contact: hover-revealed swap icon, gated on contacts:write
        because creating a fresh contact via the inline quick-create form
        needs the same perm as the full contact admin form. The conversation
        endpoint also requires conversations:update_status; relying on the
        backend perm check rather than gating the icon with an AND of both
        keeps the UI permissive (an agent who has update_status but lost
        contacts:write briefly will see a clearer error from the backend
        than a disappeared icon). -->
      <button
        v-if="!conversationStore.conversation.loading && userStore.can('contacts:write')"
        type="button"
        class="opacity-0 group-hover/contact:opacity-100 focus:opacity-100 transition-opacity text-muted-foreground hover:text-foreground"
        :title="t('conversation.changeContact')"
        :aria-label="t('conversation.changeContact')"
        @click="openChangeContact"
      >
        <ArrowRightLeft :size="14" />
      </button>
    </div>

    <!-- Change-contact picker. Search on name or email, click a result to
      reassign, or click the + icon to inline-create a new contact and assign
      it in one step. -->
    <div v-if="showContactSearch" class="space-y-2">
      <div class="relative">
        <input
          ref="contactSearchInput"
          v-model="contactQuery"
          @input="searchContacts"
          :placeholder="t('contact.searchByNameOrEmail')"
          class="w-full text-sm border rounded px-2 py-1.5 pr-12"
        />
        <div class="absolute right-1 top-1/2 -translate-y-1/2 flex items-center gap-0.5">
          <button
            type="button"
            class="text-muted-foreground hover:text-foreground p-0.5"
            :title="t('contact.createNew')"
            :aria-label="t('contact.createNew')"
            @click="showCreateForm = true"
          >
            <Plus :size="14" />
          </button>
          <button
            type="button"
            class="text-muted-foreground hover:text-foreground p-0.5"
            :title="t('globals.messages.close')"
            :aria-label="t('globals.messages.close')"
            @click="closeContactSearch"
          >
            <X :size="14" />
          </button>
        </div>
      </div>
      <div v-if="contactResults.length" class="border rounded max-h-40 overflow-y-auto">
        <button
          v-for="c in contactResults"
          :key="c.id"
          type="button"
          class="w-full text-left px-2 py-1.5 text-sm hover:bg-muted flex flex-col"
          @click="changeContact(c)"
        >
          <span>{{ c.first_name }} {{ c.last_name }}</span>
          <span class="text-xs text-muted-foreground">{{ c.email }}</span>
        </button>
      </div>
      <p
        v-if="contactQuery.length >= 2 && !contactResults.length && !searching && !showCreateForm"
        class="text-xs text-muted-foreground px-1"
      >
        {{ t('contact.noContactsFound') }}
      </p>

      <div v-if="showCreateForm" class="border rounded p-2 space-y-2 bg-muted/30">
        <p class="text-xs font-medium">{{ t('contact.createNew') }}</p>
        <input
          v-model="newContact.email"
          :placeholder="t('contact.emailPlaceholder')"
          type="email"
          class="w-full text-sm border rounded px-2 py-1"
          @keyup.escape="showCreateForm = false"
        />
        <div class="flex gap-1">
          <input
            v-model="newContact.first_name"
            :placeholder="t('globals.terms.firstName')"
            class="w-1/2 text-sm border rounded px-2 py-1"
          />
          <input
            v-model="newContact.last_name"
            :placeholder="t('globals.terms.lastName')"
            class="w-1/2 text-sm border rounded px-2 py-1"
          />
        </div>
        <div class="flex gap-1">
          <Button size="sm" class="h-7 text-xs" :disabled="!newContact.email" @click="createAndAssign">
            {{ t('contact.createAndAssign') }}
          </Button>
          <Button size="sm" variant="ghost" class="h-7 text-xs" @click="showCreateForm = false">
            {{ t('globals.messages.cancel') }}
          </Button>
        </div>
      </div>
    </div>
    <div class="flex gap-2 items-center group/email">
      <Mail size="16" class="text-muted-foreground flex-shrink-0" />
      <Tooltip v-if="isLivechat && !conversationStore.conversation.loading">
        <TooltipTrigger as-child>
          <ShieldCheck v-if="isVerified" size="14" class="flex-shrink-0 text-green-600" />
          <ShieldQuestion v-else size="14" class="flex-shrink-0 text-amber-500" />
        </TooltipTrigger>
        <TooltipContent>{{
          isVerified ? t('contact.identityVerified') : t('contact.identityNotVerified')
        }}</TooltipContent>
      </Tooltip>
      <span v-if="conversationStore.conversation.loading">
        <Skeleton class="w-32 h-4" />
      </span>
      <span v-else-if="conversation?.contact?.email" class="sidebar-value break-all">
        {{ conversation?.contact?.email }}
      </span>
      <span v-else class="sidebar-label">
        {{ t('conversation.sidebar.notAvailable') }}
      </span>
      <!-- UX10: copy-to-clipboard, hover to reveal. Hidden once the conversation
        is loading or the contact has no email. -->
      <button
        v-if="!conversationStore.conversation.loading && conversation?.contact?.email"
        type="button"
        class="flex-shrink-0 opacity-0 group-hover/email:opacity-100 focus:opacity-100 transition-opacity text-muted-foreground hover:text-foreground"
        @click="copyEmail"
        :title="emailCopied ? t('contact.emailCopied') : t('contact.copyEmail')"
      >
        <ClipboardCheck v-if="emailCopied" :size="14" class="text-green-500" />
        <Copy v-else :size="14" />
      </button>
    </div>
    <div class="flex gap-2 items-center">
      <Phone size="16" class="text-muted-foreground flex-shrink-0" />
      <span v-if="conversationStore.conversation.loading">
        <Skeleton class="w-32 h-4" />
      </span>
      <span v-else class="sidebar-value">
        {{ phoneNumber }}
      </span>
    </div>
    <div class="flex gap-2 items-center" v-if="conversation?.contact?.external_user_id">
      <IdCard size="16" class="text-muted-foreground flex-shrink-0" />
      <span v-if="conversationStore.conversation.loading">
        <Skeleton class="w-32 h-4" />
      </span>
      <span v-else class="sidebar-value">
        {{ conversation.contact.external_user_id }}
      </span>
    </div>

    <!-- Livechat visitor info -->
    <template v-if="isLivechat && !conversationStore.conversation.loading">
      <div v-if="conversation?.contact?.country" class="flex gap-2 items-center">
        <Globe size="16" class="text-muted-foreground flex-shrink-0" />
        <span class="sidebar-value">{{ countryName }}</span>
      </div>
      <div v-if="conversation?.meta?.ip" class="flex gap-2 items-center">
        <Monitor size="16" class="text-muted-foreground flex-shrink-0" />
        <span class="sidebar-value break-all">{{ conversation.meta.ip }}</span>
      </div>
      <div v-if="conversation?.meta?.user_agent" class="flex gap-2 items-center">
        <Smartphone size="16" class="text-muted-foreground flex-shrink-0" />
        <span class="sidebar-value break-all">{{ parsedUA }}</span>
      </div>
    </template>

    <!-- Context Links -->
    <template v-if="contextLinks.length > 0 && !conversationStore.conversation.loading">
      <div
        v-for="app in contextLinks"
        :key="app.id"
        class="flex gap-2 items-center cursor-pointer group"
        @click="openContextLink(app)"
      >
        <ExternalLink size="16" class="text-muted-foreground flex-shrink-0" />
        <span
          class="sidebar-value group-hover:underline"
          :class="{ 'text-muted-foreground': loadingAppId === app.id }"
        >
          {{ app.name }}
        </span>
      </div>
    </template>
  </div>
</template>

<script setup>
import { computed, ref, nextTick, onMounted } from 'vue'
import { ViewVerticalIcon } from '@radix-icons/vue'
import { Button } from '@shared-ui/components/ui/button'
import { Avatar, AvatarFallback, AvatarImage } from '@shared-ui/components/ui/avatar'
import StatusDot from '@shared-ui/components/StatusDot.vue'
import {
  Mail,
  Phone,
  ExternalLink,
  IdCard,
  Globe,
  Monitor,
  Smartphone,
  ShieldCheck,
  ShieldQuestion,
  Copy,
  ClipboardCheck,
  ArrowRightLeft,
  Plus,
  X
} from 'lucide-vue-next'
import { Tooltip, TooltipContent, TooltipTrigger } from '@shared-ui/components/ui/tooltip'
import countries from '@/constants/countries.js'
import { useEmitter } from '@/composables/useEmitter'
import { EMITTER_EVENTS } from '@/constants/emitterEvents.js'
import { useConversationStore } from '@/stores/conversation'
import { Skeleton } from '@shared-ui/components/ui/skeleton'
import { useUserStore } from '@/stores/user'
import { useI18n } from 'vue-i18n'
import api from '../../../api'
import { handleHTTPError } from '@shared-ui/utils/http.js'
const conversationStore = useConversationStore()
const emitter = useEmitter()
const conversation = computed(() => conversationStore.current)
const { t } = useI18n()
const userStore = useUserStore()

// UX18: change-contact picker state. Search by name or email (the contacts
// search query was widened in this port from email-only to email+name), pick
// from the results, or quick-create a fresh contact and assign it without
// leaving the conversation. The contact change endpoint accepts the existing
// contact id; the created contact is fetched back through the standard
// contact endpoint so the sidebar gets the same shape it already renders.
const showContactSearch = ref(false)
const contactQuery = ref('')
const contactResults = ref([])
const contactSearchInput = ref(null)
const searching = ref(false)
let searchTimeout = null
const showCreateForm = ref(false)
const newContact = ref({ email: '', first_name: '', last_name: '' })

const openChangeContact = () => {
  showContactSearch.value = true
  nextTick(() => contactSearchInput.value?.focus())
}

const closeContactSearch = () => {
  showContactSearch.value = false
  showCreateForm.value = false
  contactQuery.value = ''
  contactResults.value = []
  newContact.value = { email: '', first_name: '', last_name: '' }
}

const searchContacts = () => {
  clearTimeout(searchTimeout)
  if (contactQuery.value.length < 2) {
    contactResults.value = []
    return
  }
  searching.value = true
  searchTimeout = setTimeout(async () => {
    try {
      const res = await api.searchContacts({ query: contactQuery.value })
      contactResults.value = res.data?.data || []
    } catch {
      contactResults.value = []
    } finally {
      searching.value = false
    }
  }, 300)
}

const changeContact = async (contact) => {
  const uuid = conversation.value?.uuid
  if (!uuid || !contact?.id) return
  try {
    await api.updateConversationContact(uuid, contact.id)
    // Optimistically update the sidebar so the new contact name/email
    // appear without waiting for a refetch. The WS broadcast from the
    // backend's BroadcastConversationUpdate will overwrite contact_id on
    // other tabs but the contact object itself only updates on next fetch
    // there — same pattern as priority/subject inline edits.
    conversationStore.current.contact_id = contact.id
    conversationStore.current.contact = {
      ...conversationStore.current.contact,
      id: contact.id,
      first_name: contact.first_name,
      last_name: contact.last_name,
      email: contact.email,
      avatar_url: contact.avatar_url,
      phone_number: contact.phone_number,
      phone_number_country_code: contact.phone_number_country_code
    }
    closeContactSearch()
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      description: t('conversation.contactUpdated')
    })
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  }
}

const createAndAssign = async () => {
  if (!newContact.value.email) return
  try {
    const res = await api.quickCreateContact(newContact.value)
    const created = res.data?.data
    if (created) {
      await changeContact(created)
    }
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  }
}

// UX10: copy contact email to clipboard. Reverts the icon back to Copy after
// 2s. navigator.clipboard.writeText needs an HTTPS context, but the staging /
// production deployments are both HTTPS so we don't fall back to execCommand.
const emailCopied = ref(false)
const copyEmail = async () => {
  const email = conversation.value?.contact?.email
  if (!email) return
  try {
    await navigator.clipboard.writeText(email)
    emailCopied.value = true
    setTimeout(() => { emailCopied.value = false }, 2000)
  } catch {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: t('contact.emailCopyFailed')
    })
  }
}

const phoneNumber = computed(() => {
  const countryCodeValue = conversation.value?.contact?.phone_number_country_code || ''
  const number = conversation.value?.contact?.phone_number || t('conversation.sidebar.notAvailable')
  if (!countryCodeValue) return number

  // Lookup calling code
  const country = countries.find((c) => c.iso_2 === countryCodeValue)
  const callingCode = country ? country.calling_code : countryCodeValue
  return `${callingCode} ${number}`
})

const countryName = computed(() => {
  const code = conversation.value?.contact?.country
  if (!code) return ''
  const c = countries.find((c) => c.iso_2 === code)
  return c ? c.name : code
})

const isLivechat = computed(() => conversation.value?.inbox_channel === 'livechat')
const contactStatus = computed(() => conversation.value?.contact?.availability_status)
const isVerified = computed(
  () => isLivechat.value && conversation.value?.contact?.type !== 'visitor'
)

const parsedUA = computed(() => {
  const ua = conversation.value?.meta?.user_agent
  if (!ua) return ''
  const browser = ua.match(/(Chrome|Firefox|Safari|Edge|Opera|MSIE|Trident)[/\s](\d+)/i)
  const os = ua.match(/(Windows|Mac OS X|Linux|Android|iOS|iPhone|iPad)[\s/]?([0-9._]*)/i)
  const parts = []
  if (browser) parts.push(browser[1] + ' ' + browser[2])
  if (os) parts.push(os[1].replace('_', ' '))
  return parts.length > 0 ? parts.join(' / ') : ua.substring(0, 60)
})

const contextLinks = ref([])
const loadingAppId = ref(null)

onMounted(async () => {
  try {
    const resp = await api.getActiveContextLinks()
    contextLinks.value = resp.data.data || []
  } catch {
    // Silently ignore — context links are optional.
  }
})

const openContextLink = async (app) => {
  const uuid = conversation.value?.uuid
  if (!uuid) return
  try {
    loadingAppId.value = app.id
    const resp = await api.getContextLinkURL(app.id, uuid)
    window.open(resp.data.data, '_blank')
  } catch {
    // Silently ignore.
  } finally {
    loadingAppId.value = null
  }
}
</script>
