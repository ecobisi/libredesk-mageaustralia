<template>
  <ContextMenu>
    <ContextMenuTrigger asChild>
      <router-link
        :to="conversationRoute"
        class="group relative block px-3 py-3 transition-all duration-200 ease-in-out cursor-pointer hover:bg-accent/20 dark:hover:bg-accent/60"
        :class="{
          'bg-accent/60': conversation.uuid === currentConversation?.uuid,
          'bg-primary/5': isItemSelected && conversation.uuid !== currentConversation?.uuid
        }"
      >
        <div class="flex items-start gap-2">
          <!-- Selection checkbox -->
          <div class="flex items-center pt-2" @click.prevent.stop="handleCheckboxClick">
            <Checkbox
              :checked="isItemSelected"
              :aria-label="t('conversation.bulkActions.selectConversation')"
            />
          </div>

          <!-- Avatar with channel indicator -->
          <div class="relative flex-shrink-0">
            <Avatar class="w-10 h-10 rounded-full">
              <AvatarImage
                :src="conversation.contact.avatar_url || ''"
                class="object-cover"
              />
              <AvatarFallback>
                {{ conversation.contact.first_name.substring(0, 2).toUpperCase() }}
              </AvatarFallback>
            </Avatar>
            <span class="absolute -bottom-0.5 -right-0.5 flex items-center justify-center w-4 h-4 rounded-full bg-background border border-border">
              <component :is="conversation.inbox_channel === 'livechat' ? MessageSquare : Mail" class="w-2.5 h-2.5 text-muted-foreground" />
            </span>
          </div>

          <!-- Content container -->
          <div class="flex-1 min-w-0 space-y-2">
            <!-- Name + Subject group -->
            <div>
              <!-- Contact name + inbox + time -->
              <div class="flex items-baseline justify-between gap-2">
                <div class="flex items-baseline gap-1.5 min-w-0">
                  <h3 class="text-sm font-semibold truncate text-foreground">
                    {{ contactFullName }}
                  </h3>
                  <span class="text-xs text-muted-foreground truncate">
                    {{ conversation.inbox_name }}
                  </span>
                </div>
              </div>

              <!-- Subject -->
              <p
                v-if="conversation.subject"
                class="text-xs text-muted-foreground truncate"
              >
                {{ conversation.subject }}
              </p>
            </div>

            <!-- Message preview + unread count -->
            <div class="flex items-center justify-between gap-2">
              <TooltipProvider :delay-duration="400">
                <Tooltip>
                  <TooltipTrigger asChild>
                    <p class="text-sm flex-1 min-w-0 truncate text-muted-foreground">
                      <template v-if="hasDraftForConversation">
                        <span class="font-medium text-primary">{{ $t('globals.terms.draft') }}:</span>
                        {{ draftPreview }}
                      </template>
                      <template v-else>
                        <Reply
                          class="text-green-600 inline-block align-text-bottom mr-0.5"
                          :size="14"
                          v-if="conversation.last_message_sender === 'agent'"
                        />{{ trimmedLastMessage }}
                      </template>
                    </p>
                  </TooltipTrigger>
                  <TooltipContent
                    v-if="!hasDraftForConversation && (conversation.first_message || conversation.last_message)"
                    side="bottom"
                    align="start"
                    class="max-w-md p-3 text-xs leading-relaxed bg-popover text-popover-foreground border shadow-lg space-y-2"
                  >
                    <div v-if="conversation.first_message">
                      <p class="text-[10px] font-semibold text-muted-foreground uppercase tracking-wide mb-1">
                        {{ t('conversation.list.preview.originalMessage') }}
                      </p>
                      <p class="whitespace-pre-line line-clamp-4">{{ conversation.first_message }}</p>
                    </div>
                    <div
                      v-if="conversation.last_message && conversation.last_message !== conversation.first_message"
                      class="border-t pt-2"
                    >
                      <p
                        class="text-[10px] font-semibold uppercase tracking-wide mb-1"
                        :class="conversation.last_message_sender === 'agent' ? 'text-green-600' : 'text-muted-foreground'"
                      >
                        {{ conversation.last_message_sender === 'agent'
                          ? t('conversation.list.preview.latestReplyAgent')
                          : t('conversation.list.preview.latestReplyCustomer') }}
                      </p>
                      <p class="whitespace-pre-line line-clamp-4">{{ conversation.last_message }}</p>
                    </div>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
              <div
                v-if="conversation.unread_message_count > 0 && !canAssignAgent && !canAssignTeam"
                class="flex items-center justify-center w-5 h-5 bg-green-600 text-white text-xs font-medium rounded-full flex-shrink-0"
              >
                {{ conversation.unread_message_count > 9 ? '9+' : conversation.unread_message_count }}
              </div>
            </div>

            <!-- SLA Badges -->
            <div v-if="hasSlaDeadlines" class="flex items-center gap-1">
              <SlaBadge
                v-show="frdStatus === 'overdue' || frdStatus === 'remaining'"
                :dueAt="conversation.first_response_deadline_at"
                :actualAt="conversation.first_reply_at"
                :label="'FRD'"
                :showExtra="false"
                @status="frdStatus = $event"
                :key="`${conversation.uuid}-${conversation.first_response_deadline_at}-${conversation.first_reply_at}`"
              />
              <SlaBadge
                v-show="rdStatus === 'overdue' || rdStatus === 'remaining'"
                :dueAt="conversation.resolution_deadline_at"
                :actualAt="conversation.resolved_at"
                :label="'RD'"
                :showExtra="false"
                @status="rdStatus = $event"
                :key="`${conversation.uuid}-${conversation.resolution_deadline_at}-${conversation.resolved_at}`"
              />
              <SlaBadge
                v-show="nrdStatus === 'overdue' || nrdStatus === 'remaining'"
                :dueAt="conversation.next_response_deadline_at"
                :actualAt="conversation.next_response_met_at"
                :label="'NRD'"
                :showExtra="false"
                @status="nrdStatus = $event"
                :key="`${conversation.uuid}-${conversation.next_response_deadline_at}-${conversation.next_response_met_at}`"
              />
            </div>
          </div>

          <!-- Right column: 2x2 grid — assignments left, time+unread right -->
          <div
            v-if="canAssignAgent || canAssignTeam"
            class="flex-shrink-0 grid grid-cols-[auto_auto] gap-x-3 gap-y-1.5 items-center pt-1"
            @click.prevent.stop
          >
            <!-- Row 1: Agent | Time -->
            <DropdownMenu v-if="canAssignAgent">
              <DropdownMenuTrigger asChild>
                <button
                  class="text-xs flex items-center gap-1 py-1 px-1 justify-end hover:text-foreground transition-colors cursor-pointer"
                  :class="conversation.assigned_user_name ? 'text-muted-foreground' : 'text-orange-500 dark:text-orange-400'"
                >
                  <User class="w-3 h-3" />
                  {{ conversation.assigned_user_name || t('globals.terms.unassigned') }}
                  <ChevronDown class="w-2.5 h-2.5 opacity-50" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" class="max-h-96 overflow-y-auto">
                <DropdownMenuItem
                  v-if="conversation.assigned_user_name"
                  @click="unassignAgent"
                  class="text-xs text-muted-foreground"
                >
                  {{ t('globals.terms.none') }}
                </DropdownMenuItem>
                <DropdownMenuSeparator v-if="conversation.assigned_user_name" />
                <DropdownMenuItem
                  v-for="agent in usersStore.options"
                  :key="'agent-' + agent.value"
                  @click="assignAgent(agent)"
                  class="text-xs"
                >
                  {{ agent.label }}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
            <span v-else />
            <span class="text-xs text-gray-400 whitespace-nowrap text-right">
              {{ relativeLastMessageTime }}
            </span>

            <!-- Row 2: Team | Unread -->
            <DropdownMenu v-if="canAssignTeam">
              <DropdownMenuTrigger asChild>
                <button
                  class="text-xs flex items-center gap-1 py-1 px-1 justify-end hover:text-foreground transition-colors cursor-pointer text-muted-foreground"
                  :class="conversation.assigned_team_name ? '' : 'opacity-50'"
                >
                  <Users class="w-3 h-3" />
                  {{ conversation.assigned_team_name || t('globals.terms.noTeam') }}
                  <ChevronDown class="w-2.5 h-2.5 opacity-50" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" class="max-h-96 overflow-y-auto">
                <DropdownMenuItem
                  v-if="conversation.assigned_team_name"
                  @click="unassignTeam"
                  class="text-xs text-muted-foreground"
                >
                  {{ t('globals.terms.none') }}
                </DropdownMenuItem>
                <DropdownMenuSeparator v-if="conversation.assigned_team_name" />
                <DropdownMenuItem
                  v-for="team in teamsStore.options"
                  :key="'team-' + team.value"
                  @click="assignTeam(team)"
                  class="text-xs"
                >
                  {{ team.label }}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
            <span v-else />
            <div class="flex justify-end">
              <div
                v-if="conversation.unread_message_count > 0"
                class="flex items-center justify-center w-5 h-5 bg-green-600 text-white text-xs font-medium rounded-full"
              >
                {{ conversation.unread_message_count }}
              </div>
            </div>
          </div>
        </div>
      </router-link>
    </ContextMenuTrigger>
    <ContextMenuContent>
      <ContextMenuItem @click="handleMarkAsUnread">
        <MailOpen class="w-4 h-4 mr-2" />
        {{ $t('globals.messages.markAsUnread') }}
      </ContextMenuItem>
    </ContextMenuContent>
  </ContextMenu>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { getRelativeTime } from '@shared-ui/utils/datetime.js'
import { Mail, MessageSquare, Reply, MailOpen, User, Users, ChevronDown } from 'lucide-vue-next'
import { Avatar, AvatarFallback, AvatarImage } from '@shared-ui/components/ui/avatar'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger
} from '@shared-ui/components/ui/context-menu'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@shared-ui/components/ui/dropdown-menu'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger
} from '@shared-ui/components/ui/tooltip'
import SlaBadge from '@main/features/sla/SlaBadge.vue'
import { Checkbox } from '@shared-ui/components/ui/checkbox'
import { useConversationStore } from '@main/stores/conversation'
import { useConversationRoute } from '@main/composables/useConversationRoute'
import { useUsersStore } from '@/stores/users'
import { useTeamStore } from '@/stores/team'
import { useUserStore } from '@/stores/user'
import { useToast } from '@/composables/useToast'
import { permissions as p } from '@/constants/permissions'
import api from '@/api'
import { useI18n } from 'vue-i18n'

let timer = null
const now = ref(new Date())
const conversationStore = useConversationStore()
const usersStore = useUsersStore()
const teamsStore = useTeamStore()
const userStore = useUserStore()
const toast = useToast()
const { t } = useI18n()
const frdStatus = ref('')
const rdStatus = ref('')
const nrdStatus = ref('')

const props = defineProps({
  conversation: Object,
  currentConversation: Object,
  contactFullName: String
})

const handleMarkAsUnread = () => {
  conversationStore.markAsUnread(props.conversation.uuid)
}

const { buildConversationRoute } = useConversationRoute()
const conversationRoute = computed(() => buildConversationRoute(props.conversation))

onMounted(() => {
  timer = setInterval(() => {
    now.value = new Date()
  }, 60000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})

const trimmedLastMessage = computed(() => {
  const message = props.conversation.last_message || ''
  return message.length > 120 ? message.slice(0, 120) + '...' : message
})

const relativeLastMessageTime = computed(() => {
  return props.conversation.last_message_at
    ? getRelativeTime(props.conversation.last_message_at, now.value)
    : ''
})

const hasSlaDeadlines = computed(() => {
  const c = props.conversation
  return c.first_response_deadline_at || c.resolution_deadline_at || c.next_response_deadline_at
})

const hasDraftForConversation = computed(() => {
  return conversationStore.hasDraft(props.conversation.uuid)
})

const draftPreview = computed(() => {
  const draft = conversationStore.getDraft(props.conversation.uuid)
  if (!draft?.content) return ''
  const text = draft.content.replace(/<[^>]*>/g, '').trim()
  return text.length > 120 ? text.slice(0, 120) + '...' : text
})

const isItemSelected = computed(() => {
  return conversationStore.isSelected(props.conversation.uuid)
})

const handleCheckboxClick = (event) => {
  conversationStore.toggleSelect(props.conversation.uuid, event.shiftKey)
}

const canAssignAgent = computed(() => userStore.can(p.CONVERSATIONS_UPDATE_USER_ASSIGNEE))
const canAssignTeam = computed(() => userStore.can(p.CONVERSATIONS_UPDATE_TEAM_ASSIGNEE))

const assignAgent = async (agent) => {
  try {
    await api.updateAssignee(props.conversation.uuid, 'user', { assignee_id: parseInt(agent.value) })
    props.conversation.assigned_user_name = agent.label
  } catch (error) {
    toast.error(error)
  }
}

const unassignAgent = async () => {
  try {
    await api.removeAssignee(props.conversation.uuid, 'user')
    props.conversation.assigned_user_name = null
  } catch (error) {
    toast.error(error)
  }
}

const assignTeam = async (team) => {
  try {
    await api.updateAssignee(props.conversation.uuid, 'team', { assignee_id: parseInt(team.value) })
    props.conversation.assigned_team_name = team.label
  } catch (error) {
    toast.error(error)
  }
}

const unassignTeam = async () => {
  try {
    await api.removeAssignee(props.conversation.uuid, 'team')
    props.conversation.assigned_team_name = null
  } catch (error) {
    toast.error(error)
  }
}
</script>
