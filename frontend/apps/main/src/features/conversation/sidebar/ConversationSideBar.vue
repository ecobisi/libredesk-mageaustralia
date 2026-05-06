<template>
  <div>
    <ConversationSideBarContact class="p-4" />
    <Accordion type="multiple" collapsible v-model="accordionState">
      <AccordionItem value="actions" class="accordion-item">
        <AccordionTrigger class="accordion-trigger">
          {{ $t('globals.terms.action', 2) }}
        </AccordionTrigger>

        <!-- Agent, team, priority, and tags assignment -->
        <AccordionContent class="accordion-content--actions">
          <div>
            <SelectComboBox
              v-model="conversationStore.current.assigned_user_id"
              :items="[{ value: 'none', label: t('globals.terms.none') }, ...usersStore.options]"
              :placeholder="t('placeholders.selectAgent')"
              @select="selectAgent"
              type="user"
            />
          </div>

          <div>
            <SelectComboBox
              v-model="conversationStore.current.assigned_team_id"
              :items="[{ value: 'none', label: t('globals.terms.none') }, ...teamsStore.options]"
              :placeholder="t('placeholders.selectTeam')"
              @select="selectTeam"
              type="team"
            />
          </div>

          <div>
            <SelectComboBox
              v-model="conversationStore.current.priority_id"
              :items="priorityOptions"
              :placeholder="t('placeholders.selectPriority')"
              @select="selectPriority"
              type="priority"
            />
          </div>

          <div>
            <SelectTag
              v-if="conversationStore.current"
              v-model="conversationStore.current.tags"
              :items="tags.map((tag) => ({ label: tag, value: tag }))"
              :placeholder="t('placeholders.selectTags')"
            />
          </div>

          <!--
            UX5: followers picker. Reuses the same multi-select primitive as
            tags. SelectTag emits the new full array on add/remove; the
            computed setter below diffs against the previous array and
            issues a single add or remove API call so the backend stays
            authoritative for the participant list (and the "you were
            added as a follower" notification only fires for genuine
            additions).
          -->
          <div>
            <SelectTag
              v-if="conversationStore.current"
              v-model="followerIds"
              :items="followerOptions"
              :placeholder="t('placeholders.selectFollowers')"
              name="followers"
            />
          </div>
        </AccordionContent>
      </AccordionItem>

      <!-- Information -->
      <AccordionItem value="information" class="accordion-item">
        <AccordionTrigger class="accordion-trigger">
          {{ $t('conversation.sidebar.information') }}
        </AccordionTrigger>
        <AccordionContent class="accordion-content">
          <ConversationInfo />
        </AccordionContent>
      </AccordionItem>

      <!-- Contact attributes -->
      <AccordionItem
        value="contact_attributes"
        class="accordion-item"
        v-if="customAttributeStore.contactAttributeOptions.length > 0"
      >
        <AccordionTrigger class="accordion-trigger">
          {{ $t('conversation.sidebar.contactAttributes') }}
        </AccordionTrigger>
        <AccordionContent class="accordion-content">
          <CustomAttributes
            :loading="conversationStore.current.loading"
            :attributes="customAttributeStore.contactAttributeOptions"
            :customAttributes="conversationStore.current?.contact?.custom_attributes || {}"
            @update:setattributes="updateContactCustomAttributes"
          />
        </AccordionContent>
      </AccordionItem>

      <!-- Page visits (livechat only) -->
      <AccordionItem
        value="page_visits"
        class="accordion-item"
        v-if="conversationStore.current?.inbox_channel === 'livechat'"
      >
        <AccordionTrigger class="accordion-trigger">
          {{ $t('conversation.sidebar.lastVisitedPages') }}
        </AccordionTrigger>
        <AccordionContent class="accordion-content">
          <ConversationSideBarPageVisits />
        </AccordionContent>
      </AccordionItem>

      <!-- Previous conversations -->
      <AccordionItem value="previous_conversations" class="accordion-item">
        <AccordionTrigger class="accordion-trigger">
          {{ $t('conversation.sidebar.previousConvo') }}
        </AccordionTrigger>
        <AccordionContent class="accordion-content">
          <PreviousConversations />
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  </div>
</template>

<script setup>
import { ref, onMounted, watch, computed } from 'vue'
import { useConversationStore } from '@/stores/conversation'
import { useUsersStore } from '@/stores/users'
import { useTeamStore } from '@/stores/team'
import { useTagStore } from '@/stores/tag'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger
} from '@shared-ui/components/ui/accordion'
import ConversationInfo from './ConversationInfo.vue'
import ConversationSideBarContact from '@/features/conversation/sidebar/ConversationSideBarContact.vue'
import { SelectTag } from '@shared-ui/components/ui/select'
import { useToast } from '../../../composables/useToast'
import { useI18n } from 'vue-i18n'
import { useStorage } from '@vueuse/core'
import CustomAttributes from '@/features/conversation/sidebar/CustomAttributes.vue'
import { useCustomAttributeStore } from '../../../stores/customAttributes'
import PreviousConversations from '@/features/conversation/sidebar/PreviousConversations.vue'
import ConversationSideBarPageVisits from '@/features/conversation/sidebar/ConversationSideBarPageVisits.vue'
import SelectComboBox from '@main/components/combobox/SelectCombobox.vue'
import api from '../../../api'

const customAttributeStore = useCustomAttributeStore()
const toast = useToast()
const conversationStore = useConversationStore()
const usersStore = useUsersStore()
const teamsStore = useTeamStore()
const tagStore = useTagStore()
const tags = ref([])
// UX5: followers picker state. Server is authoritative — we hold a local
// list of {id, first_name, last_name} for label rendering and diff against
// the SelectTag v-model array of stringified IDs to determine adds vs
// removes. `syncing` guards re-entry while the add/remove POST is in
// flight so the SelectTag's optimistic emit doesn't fight the server
// response.
const followers = ref([])
const syncingFollowers = ref(false)
// Save the accordion state in local storage
const accordionState = useStorage('conversation-sidebar-accordion', ['previous_conversations'])
const { t } = useI18n()
let isConversationChange = false
customAttributeStore.fetchCustomAttributes()

// Watch for changes in the current conversation and set the flag
watch(
  () => conversationStore.current,
  (newConversation, oldConversation) => {
    // Set the flag when the conversation changes
    if (newConversation?.uuid !== oldConversation?.uuid) {
      isConversationChange = true
      fetchFollowers()
    }
  },
  { immediate: true }
)

onMounted(async () => {
  await fetchTags()
})

// Watch for changes in the tags and upsert the tags
watch(
  () => conversationStore.current?.tags,
  (newTags, oldTags) => {
    // Skip if the tags change is due to a conversation change.
    if (isConversationChange) {
      isConversationChange = false
      return
    }

    // Skip if the tags are the same (deep comparison)
    if (
      Array.isArray(newTags) &&
      Array.isArray(oldTags) &&
      newTags.length === oldTags.length &&
      newTags.every((item) => oldTags.includes(item))
    ) {
      return
    }

    conversationStore.upsertTags({
      tags: newTags
    })
  },
  { immediate: false }
)

const priorityOptions = computed(() => conversationStore.priorityOptions)

// UX5: agent options for the followers picker. Filters out the System user
// (a synthetic actor used for activity-log entries — adding it as a watcher
// would never fire a real notification) and any user already following so
// the dropdown only shows genuinely-addable agents.
const followerOptions = computed(() => {
  const opts = (usersStore.options || [])
    .filter((o) => {
      const label = (o.label || '').toLowerCase().trim()
      return label !== 'system' && label !== 'system user'
    })
    .map((o) => ({ label: o.label, value: String(o.value) }))
  return opts
})

// SelectTag binds an array of stringified IDs. The computed setter diffs
// the new array against `followers.value` to figure out which single
// agent was added or removed and dispatches the matching API call.
const followerIds = computed({
  get: () => followers.value.map((f) => String(f.id)),
  set: (newIds) => {
    if (syncingFollowers.value) return
    const oldIds = followers.value.map((f) => String(f.id))
    const added = newIds.filter((id) => !oldIds.includes(id))
    const removed = oldIds.filter((id) => !newIds.includes(id))
    // SelectTag only ever adds or removes one tag at a time, but defend
    // against both being non-empty by handling each list. The server
    // returns the refreshed participant set so the next emit sees the
    // truthful baseline.
    added.forEach((id) => addFollower(id))
    removed.forEach((id) => removeFollower(id))
  }
})

const fetchFollowers = async () => {
  const uuid = conversationStore.current?.uuid
  if (!uuid) return
  try {
    const res = await api.getConversationParticipants(uuid)
    // Backend already filters to type='agent' (UX5 query change), but the
    // System user is still type='agent' so strip it client-side too.
    followers.value = (res.data?.data || []).filter((f) => {
      const name = ((f.first_name || '') + ' ' + (f.last_name || '')).toLowerCase().trim()
      return name !== 'system' && name !== 'system user'
    })
  } catch {
    // Non-fatal — sidebar shows an empty followers picker.
  }
}

const addFollower = async (userId) => {
  const uuid = conversationStore.current?.uuid
  if (!uuid || !userId) return
  syncingFollowers.value = true
  try {
    const res = await api.addConversationFollower(uuid, userId)
    followers.value = (res.data?.data || []).filter((f) => {
      const name = ((f.first_name || '') + ' ' + (f.last_name || '')).toLowerCase().trim()
      return name !== 'system' && name !== 'system user'
    })
  } catch (error) {
    toast.error(error)
  } finally {
    syncingFollowers.value = false
  }
}

const removeFollower = async (userId) => {
  const uuid = conversationStore.current?.uuid
  if (!uuid || !userId) return
  syncingFollowers.value = true
  try {
    const res = await api.removeConversationFollower(uuid, userId)
    followers.value = (res.data?.data || []).filter((f) => {
      const name = ((f.first_name || '') + ' ' + (f.last_name || '')).toLowerCase().trim()
      return name !== 'system' && name !== 'system user'
    })
  } catch (error) {
    toast.error(error)
  } finally {
    syncingFollowers.value = false
  }
}

const fetchTags = async () => {
  await tagStore.fetchTags()
  tags.value = tagStore.tags.map((item) => item.name)
}

const handleAssignedUserChange = (id) => {
  conversationStore.updateAssignee('user', {
    assignee_id: parseInt(id)
  })
}

const handleAssignedTeamChange = (id) => {
  conversationStore.updateAssignee('team', {
    assignee_id: parseInt(id)
  })
}

const handleRemoveAssignee = (type) => {
  conversationStore.removeAssignee(type)
}

const handlePriorityChange = (priority) => {
  conversationStore.updatePriority(priority)
}

const selectAgent = (agent) => {
  if (agent.value === 'none') {
    handleRemoveAssignee('user')
    return
  }
  conversationStore.current.assigned_user_id = agent.value
  handleAssignedUserChange(agent.value)
}

const selectTeam = (team) => {
  if (team.value === 'none') {
    handleRemoveAssignee('team')
    return
  }
  handleAssignedTeamChange(team.value)
}

const selectPriority = (priority) => {
  conversationStore.current.priority = priority.label
  conversationStore.current.priority_id = priority.value
  handlePriorityChange(priority.label)
}

const updateContactCustomAttributes = async (attributes) => {
  let previousAttributes = conversationStore.current.contact.custom_attributes
  try {
    conversationStore.current.contact.custom_attributes = attributes
    await api.updateContactCustomAttribute(conversationStore.current.uuid, attributes)
    toast.success(t('globals.messages.savedSuccessfully'))
  } catch (error) {
    toast.error(error)
    conversationStore.current.contact.custom_attributes = previousAttributes
  }
}
</script>

<style scoped>
:deep(.accordion-item) {
  @apply border-0 mb-2;
}

:deep(.accordion-trigger) {
  @apply bg-muted p-2 text-sm font-medium rounded mx-2;
}

:deep(.accordion-content) {
  @apply p-4;
}

:deep(.accordion-content--actions) {
  @apply space-y-3 p-4;
}
</style>
