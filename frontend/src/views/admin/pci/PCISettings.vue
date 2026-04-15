<script setup>
import { ref, onMounted } from 'vue'
import { toast } from 'vue-sonner'
import api from '@/api'
import AdminPageWithHelp from '@/layouts/admin/AdminPageWithHelp.vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import Spinner from '@/components/ui/spinner/Spinner.vue'
import { ShieldAlert } from 'lucide-vue-next'

const loading = ref(true)
const saving = ref(false)

const notifyAgentId = ref(0)
const notifyMethod = ref('both')
const ignoreSubjects = ref('')
const agents = ref([])

onMounted(async () => {
  await Promise.all([fetchSettings(), fetchAgents()])
})

async function fetchAgents() {
  try {
    const res = await api.getUsersCompact()
    agents.value = res.data?.data || []
  } catch (err) {
    console.error('Failed to load agents', err)
  }
}

async function fetchSettings() {
  loading.value = true
  try {
    const res = await api.getSettings('pci')
    const data = res.data?.data || {}
    if (data['pci.notify_agent_id']) {
      notifyAgentId.value = data['pci.notify_agent_id']
    }
    if (data['pci.notify_method']) {
      notifyMethod.value = data['pci.notify_method']
    }
    if (Array.isArray(data['pci.ignore_subjects'])) {
      ignoreSubjects.value = data['pci.ignore_subjects'].join('\n')
    }
  } catch (err) {
    console.error('Failed to load PCI settings', err)
  } finally {
    loading.value = false
  }
}

async function saveSettings() {
  saving.value = true
  try {
    await api.updateSettings('pci', {
      'pci.notify_agent_id': parseInt(notifyAgentId.value) || 0,
      'pci.notify_method': notifyMethod.value || 'both',
      'pci.ignore_subjects': ignoreSubjects.value
        .split('\n')
        .map(s => s.trim())
        .filter(s => s.length > 0)
    })
    toast.success('PCI settings saved')
  } catch (err) {
    toast.error(err.response?.data?.message || 'Failed to save settings')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <AdminPageWithHelp>
    <template #content>
      <div v-if="loading" class="flex justify-center py-12">
        <Spinner />
      </div>

      <div v-else class="space-y-6">
        <Card>
          <CardHeader>
            <div class="flex items-center gap-2">
              <ShieldAlert class="h-5 w-5" />
              <CardTitle>PCI Redaction</CardTitle>
            </div>
            <CardDescription>
              Configure notifications when PCI card data is detected and the original email cannot be automatically deleted from Gmail.
            </CardDescription>
          </CardHeader>
          <CardContent class="space-y-6">
            <div class="space-y-2">
              <Label>Notify agent on IMAP delete failure</Label>
              <Select v-model="notifyAgentId">
                <SelectTrigger class="max-w-xs">
                  <SelectValue placeholder="Select an agent..." />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem :value="0">None (disabled)</SelectItem>
                  <SelectItem
                    v-for="agent in agents"
                    :key="agent.id"
                    :value="agent.id"
                  >
                    {{ agent.first_name }} {{ agent.last_name }}
                  </SelectItem>
                </SelectContent>
              </Select>
              <p class="text-xs text-muted-foreground">
                This person will be notified when card data is redacted but the original email could not be deleted from Gmail.
              </p>
            </div>

            <div class="space-y-2">
              <Label>Notification method</Label>
              <Select v-model="notifyMethod">
                <SelectTrigger class="max-w-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="in_app">In-app only</SelectItem>
                  <SelectItem value="email">Email only</SelectItem>
                  <SelectItem value="both">Both (in-app + email)</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div class="space-y-2">
              <Label>Skip PCI scan for these subjects</Label>
              <Textarea
                v-model="ignoreSubjects"
                class="max-w-2xl font-mono text-sm"
                rows="5"
                placeholder="You have received a payment for Invoice&#10;Stripe receipt"
              />
              <p class="text-xs text-muted-foreground">
                One pattern per line. Incoming messages whose subject contains any of these (case-insensitive) will skip credit-card scanning entirely. Useful for payment-processor notifications that legitimately contain masked card digits.
              </p>
            </div>

            <div class="pt-4">
              <Button @click="saveSettings" :disabled="saving">
                {{ saving ? 'Saving...' : 'Save' }}
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    </template>
    <template #help>
      <h4 class="font-medium mb-2">How PCI Redaction Works</h4>
      <ul class="text-sm text-muted-foreground list-disc list-inside space-y-1">
        <li>Incoming messages are scanned for credit card numbers on arrival</li>
        <li>A warning banner appears on messages with detected card data</li>
        <li>Agents can manually redact immediately via the "Redact Now" button</li>
        <li>After 7 days, card data is automatically redacted</li>
        <li>Notification emails to agents always have card numbers masked</li>
      </ul>
      <h4 class="font-medium mt-4 mb-2">IMAP Deletion</h4>
      <p class="text-sm text-muted-foreground mb-2">
        When a message is redacted, the system attempts to delete the original email from Gmail via IMAP.
        If this fails (e.g. IMAP credentials expired), the configured agent is notified so they can manually delete it.
      </p>
      <h4 class="font-medium mt-4 mb-2">Tips</h4>
      <ul class="text-sm text-muted-foreground list-disc list-inside space-y-1">
        <li>Set the notify agent to an admin who has Gmail access</li>
        <li>Card numbers in notification emails are always masked regardless of these settings</li>
      </ul>
    </template>
  </AdminPageWithHelp>
</template>
