<!--
  EcommerceSettings — admin page for ecommerce platform integration.

  T3p port from v1.0.3 810770de end-state. Two cards:

  1. Provider config: type dropdown (magento1 / magento2 / shopify),
     base URL, Client ID + Client Secret. Test Connection button posts
     the in-flight form to /api/v1/ecommerce/test so the admin can
     verify before saving. Save button persists via PUT
     /api/v1/ecommerce/settings; the backend re-runs initEcommerceManager
     so a fresh save takes effect immediately without restart.

  2. Test Integration: only visible when a provider is configured.
     Two lookup forms (customer-by-email, order-by-number) hit the
     /api/v1/ecommerce/test/customer and /api/v1/ecommerce/test/order
     diagnostic endpoints. Customer result panel surfaces the
     `warnings` array in a destructive-coloured box at the top
     (T3ae UI fix — without this surfacing, 401s and other auth
     failures get swallowed and the admin sees a misleading "no
     customer found" message).

  Provider config + secret encryption is handled server-side via
  the encryptedFields registry; the secret field renders as
  type=password and only sends a value when the admin types one
  (empty string => keep existing).
-->
<template>
  <AdminSplitLayout>
    <template #content>
      <div :class="{ 'opacity-50 transition-opacity duration-300': isLoading }" class="space-y-6">
        <!-- Provider configuration -->
        <Card>
          <CardHeader>
            <div class="flex items-center gap-2">
              <ShoppingCart class="h-5 w-5" />
              <CardTitle>{{ t('admin.ecommerce.title') }}</CardTitle>
              <Badge v-if="hasConfig && providerType" variant="secondary">
                {{ providerName }}
              </Badge>
              <Badge v-if="testStatus === 'success'" class="bg-green-100 text-green-800">
                <CheckCircle class="h-3 w-3 mr-1" />
                {{ t('admin.ecommerce.connected') }}
              </Badge>
            </div>
            <CardDescription>{{ t('admin.ecommerce.description') }}</CardDescription>
          </CardHeader>
          <CardContent class="space-y-6">
            <div class="space-y-2">
              <Label for="ecommerce-provider">{{ t('admin.ecommerce.provider') }}</Label>
              <Select v-model="providerType">
                <SelectTrigger id="ecommerce-provider">
                  <SelectValue :placeholder="t('admin.ecommerce.selectProvider')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="disabled">{{ t('admin.ecommerce.disabled') }}</SelectItem>
                  <SelectItem value="magento1">{{ t('admin.ecommerce.providerMagento1') }}</SelectItem>
                  <SelectItem value="magento2">{{ t('admin.ecommerce.providerMagento2') }}</SelectItem>
                  <SelectItem value="shopify">{{ t('admin.ecommerce.providerShopify') }}</SelectItem>
                </SelectContent>
              </Select>
              <p class="text-xs text-muted-foreground">{{ t('admin.ecommerce.providerHelp') }}</p>
            </div>

            <template v-if="providerType && providerType !== 'disabled'">
              <div class="space-y-2">
                <Label for="ecommerce-base-url">{{ t('admin.ecommerce.baseURL') }}</Label>
                <Input
                  id="ecommerce-base-url"
                  v-model="baseURL"
                  :placeholder="baseURLPlaceholder"
                />
                <p class="text-xs text-muted-foreground">{{ baseURLHelp }}</p>
              </div>

              <div class="space-y-2">
                <Label for="ecommerce-client-id">{{ clientIDLabel }}</Label>
                <Input
                  id="ecommerce-client-id"
                  v-model="clientID"
                  :placeholder="hasConfig ? t('admin.ecommerce.configured') : t('admin.ecommerce.clientIDPlaceholder')"
                />
                <p class="text-xs text-muted-foreground">{{ clientIDHelp }}</p>
              </div>

              <div class="space-y-2">
                <Label for="ecommerce-client-secret">{{ clientSecretLabel }}</Label>
                <Input
                  id="ecommerce-client-secret"
                  v-model="clientSecret"
                  type="password"
                  :placeholder="hasConfig ? t('admin.ecommerce.clientSecretChange') : t('admin.ecommerce.clientSecretPlaceholder')"
                />
                <p class="text-xs text-muted-foreground">{{ clientSecretHelp }}</p>
              </div>
            </template>

            <div class="flex gap-2 flex-wrap pt-2">
              <Button @click="saveSettings" :disabled="saving">
                {{ saving ? t('globals.messages.saving') : t('globals.messages.save') }}
              </Button>
              <Button
                v-if="providerType && providerType !== 'disabled'"
                variant="outline"
                @click="testConnection"
                :disabled="testing || !baseURL"
              >
                <RefreshCw v-if="testing" class="h-4 w-4 mr-2 animate-spin" />
                <CheckCircle v-else-if="testStatus === 'success'" class="h-4 w-4 mr-2 text-green-500" />
                <AlertCircle v-else-if="testStatus === 'error'" class="h-4 w-4 mr-2 text-red-500" />
                {{ t('admin.ecommerce.testConnection') }}
              </Button>
              <Button
                v-if="hasConfig && providerType && providerType !== 'disabled'"
                variant="destructive"
                @click="clearSettings"
              >
                {{ t('admin.ecommerce.disable') }}
              </Button>
            </div>
          </CardContent>
        </Card>

        <!-- Test integration: customer + order lookup -->
        <Card v-if="hasConfig && providerType && providerType !== 'disabled'">
          <CardHeader>
            <CardTitle>{{ t('admin.ecommerce.testIntegrationTitle') }}</CardTitle>
            <CardDescription>{{ t('admin.ecommerce.testIntegrationDescription') }}</CardDescription>
          </CardHeader>
          <CardContent class="space-y-6">
            <!-- Customer lookup -->
            <div class="space-y-2">
              <Label>{{ t('admin.ecommerce.lookupCustomerLabel') }}</Label>
              <div class="flex gap-2">
                <Input
                  v-model="testEmail"
                  :placeholder="t('admin.ecommerce.lookupCustomerPlaceholder')"
                  class="flex-1"
                />
                <Button
                  @click="testCustomerLookup"
                  :disabled="testingCustomer || !testEmail"
                  variant="outline"
                >
                  {{ testingCustomer ? t('admin.ecommerce.lookingUp') : t('admin.ecommerce.lookup') }}
                </Button>
              </div>
              <div v-if="customerResult" class="mt-2 p-3 bg-muted rounded-md text-sm">
                <!-- T3ae warnings surfacing — auth/permission failures are
                     reported as warnings (not errors) so the response body
                     still has a 200 envelope. Render them prominently so
                     admins can distinguish "no customer with that email"
                     from "auth broke and we couldn't even check". -->
                <div
                  v-if="customerResult.warnings && customerResult.warnings.length"
                  class="mb-2 p-2 bg-destructive/10 border border-destructive/30 rounded text-destructive"
                >
                  <strong>{{ t('admin.ecommerce.lookupError') }}</strong>
                  <ul class="list-disc list-inside ml-2">
                    <li v-for="(w, i) in customerResult.warnings" :key="i">{{ w }}</li>
                  </ul>
                </div>
                <div v-if="customerResult.customer">
                  <strong>{{ t('admin.ecommerce.customer') }}:</strong>
                  {{ customerResult.customer.first_name }} {{ customerResult.customer.last_name }}
                  ({{ customerResult.customer.email }})
                </div>
                <div v-if="customerResult.recent_orders && customerResult.recent_orders.length">
                  <strong>{{ t('admin.ecommerce.recentOrders') }}:</strong>
                  <ul class="list-disc list-inside ml-2">
                    <li v-for="order in customerResult.recent_orders" :key="order.id">
                      #{{ order.increment_id }} - {{ order.status }} - ${{ order.grand_total?.toFixed(2) }}
                    </li>
                  </ul>
                </div>
                <div
                  v-if="
                    !customerResult.customer &&
                    (!customerResult.recent_orders || !customerResult.recent_orders.length) &&
                    (!customerResult.warnings || !customerResult.warnings.length)
                  "
                >
                  {{ t('admin.ecommerce.noCustomer') }}
                </div>
              </div>
            </div>

            <!-- Order lookup -->
            <div class="space-y-2">
              <Label>{{ t('admin.ecommerce.lookupOrderLabel') }}</Label>
              <div class="flex gap-2">
                <Input
                  v-model="testOrderNumber"
                  :placeholder="t('admin.ecommerce.lookupOrderPlaceholder')"
                  class="flex-1"
                />
                <Button
                  @click="testOrderLookup"
                  :disabled="testingOrder || !testOrderNumber"
                  variant="outline"
                >
                  {{ testingOrder ? t('admin.ecommerce.lookingUp') : t('admin.ecommerce.lookup') }}
                </Button>
              </div>
              <div v-if="orderResult" class="mt-2 p-3 bg-muted rounded-md text-sm">
                <div><strong>{{ t('admin.ecommerce.order') }} #{{ orderResult.increment_id }}</strong></div>
                <div>{{ t('admin.ecommerce.status') }}: {{ orderResult.status }}</div>
                <div>
                  {{ t('admin.ecommerce.customer') }}:
                  {{ orderResult.customer_name }} ({{ orderResult.customer_email }})
                </div>
                <div>{{ t('admin.ecommerce.total') }}: ${{ orderResult.grand_total?.toFixed(2) }}</div>
                <div v-if="orderResult.items && orderResult.items.length">
                  <strong>{{ t('admin.ecommerce.items') }}:</strong>
                  <ul class="list-disc list-inside ml-2">
                    <li v-for="item in orderResult.items" :key="item.sku">
                      {{ item.name }} ({{ item.sku }}) x{{ item.qty }}
                    </li>
                  </ul>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </template>
    <template #help>
      <p>{{ t('admin.ecommerce.help') }}</p>
    </template>
  </AdminSplitLayout>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AdminSplitLayout from '@/layouts/admin/AdminSplitLayout.vue'
import { Button } from '@shared-ui/components/ui/button'
import { Label } from '@shared-ui/components/ui/label'
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
import { ShoppingCart, CheckCircle, AlertCircle, RefreshCw } from 'lucide-vue-next'
import { useEmitter } from '@/composables/useEmitter'
import { EMITTER_EVENTS } from '@/constants/emitterEvents'
import api from '@/api'

const { t } = useI18n()
const emitter = useEmitter()

const isLoading = ref(true)
const saving = ref(false)
const testing = ref(false)

// Provider form state. providerType === '' / 'disabled' both mean "no
// integration"; the explicit 'disabled' option exists so the Select
// shows a real choice rather than an empty placeholder when the admin
// wants to turn the integration off.
const providerType = ref('')
const baseURL = ref('')
const clientID = ref('')
const clientSecret = ref('')

// Status tracking: hasConfig flips to true once a provider is loaded
// from the backend so the secret placeholder reads "Enter new secret
// to change" and the lookup card renders. testStatus is the most
// recent Test Connection result for the badge + button icon.
const hasConfig = ref(false)
const testStatus = ref(null)

// Lookup form state.
const testEmail = ref('')
const testOrderNumber = ref('')
const testingCustomer = ref(false)
const testingOrder = ref(false)
const customerResult = ref(null)
const orderResult = ref(null)

const showToast = (description, variant) =>
  emitter.emit(EMITTER_EVENTS.SHOW_TOAST, variant ? { variant, description } : { description })

const providerName = computed(() => {
  switch (providerType.value) {
    case 'magento1':
      return t('admin.ecommerce.providerMagento1Short')
    case 'magento2':
      return t('admin.ecommerce.providerMagento2Short')
    case 'shopify':
      return t('admin.ecommerce.providerShopifyShort')
    default:
      return ''
  }
})

const baseURLPlaceholder = computed(() =>
  providerType.value === 'shopify'
    ? 'https://your-store.myshopify.com'
    : 'https://your-store.com'
)

const baseURLHelp = computed(() => {
  switch (providerType.value) {
    case 'shopify':
      return t('admin.ecommerce.baseURLHelpShopify')
    case 'magento1':
      return t('admin.ecommerce.baseURLHelpMagento1')
    default:
      return t('admin.ecommerce.baseURLHelpMagento2')
  }
})

const clientIDLabel = computed(() => {
  switch (providerType.value) {
    case 'shopify':
      return t('admin.ecommerce.shopifyApiKey')
    case 'magento1':
      return t('admin.ecommerce.clientID')
    default:
      return t('admin.ecommerce.magento2AccessToken')
  }
})

const clientIDHelp = computed(() => {
  switch (providerType.value) {
    case 'shopify':
      return t('admin.ecommerce.shopifyApiKeyHelp')
    case 'magento1':
      return t('admin.ecommerce.clientIDHelp')
    default:
      return t('admin.ecommerce.magento2AccessTokenHelp')
  }
})

const clientSecretLabel = computed(() => {
  switch (providerType.value) {
    case 'shopify':
      return t('admin.ecommerce.shopifySecretKey')
    case 'magento1':
      return t('admin.ecommerce.clientSecret')
    default:
      return t('admin.ecommerce.magento2IntegrationSecret')
  }
})

const clientSecretHelp = computed(() => {
  switch (providerType.value) {
    case 'shopify':
      return t('admin.ecommerce.shopifySecretKeyHelp')
    case 'magento1':
      return t('admin.ecommerce.clientSecretHelp')
    default:
      return t('admin.ecommerce.magento2IntegrationSecretHelp')
  }
})

onMounted(async () => {
  await fetchSettings()
})

async function fetchSettings() {
  isLoading.value = true
  try {
    const res = await api.getEcommerceSettings()
    const data = res.data?.data
    if (data?.type) {
      providerType.value = data.type
      baseURL.value = data.base_url || ''
      clientID.value = data.client_id || ''
      hasConfig.value = true
    }
  } catch (err) {
    // Not configured yet — leave defaults. Don't toast: 404/empty here
    // is the normal state for a fresh install.
  } finally {
    isLoading.value = false
  }
}

async function saveSettings() {
  if (
    providerType.value &&
    providerType.value !== 'disabled' &&
    !baseURL.value
  ) {
    showToast(t('admin.ecommerce.baseURLRequired'), 'destructive')
    return
  }
  saving.value = true
  try {
    // 'disabled' is a UI-only sentinel that maps to an empty type on
    // the backend (clears the integration).
    const payloadType = providerType.value === 'disabled' ? '' : providerType.value
    await api.updateEcommerceSettings({
      type: payloadType,
      base_url: baseURL.value,
      client_id: clientID.value,
      client_secret: clientSecret.value
    })
    showToast(t('globals.messages.savedSuccessfully'))
    // Clear the secret input after save so it doesn't sit in the form.
    clientSecret.value = ''
    hasConfig.value = !!payloadType
    testStatus.value = null
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  } finally {
    saving.value = false
  }
}

async function testConnection() {
  testing.value = true
  testStatus.value = null
  try {
    await api.testEcommerceConnection({
      type: providerType.value,
      base_url: baseURL.value,
      client_id: clientID.value,
      client_secret: clientSecret.value
    })
    showToast(t('admin.ecommerce.connectionSuccess'))
    testStatus.value = 'success'
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
    testStatus.value = 'error'
  } finally {
    testing.value = false
  }
}

function clearSettings() {
  providerType.value = ''
  baseURL.value = ''
  clientID.value = ''
  clientSecret.value = ''
  testStatus.value = null
}

async function testCustomerLookup() {
  if (!testEmail.value) return
  testingCustomer.value = true
  customerResult.value = null
  try {
    const resp = await api.testEcommerceCustomer(testEmail.value)
    customerResult.value = resp.data?.data || {}
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  } finally {
    testingCustomer.value = false
  }
}

async function testOrderLookup() {
  if (!testOrderNumber.value) return
  testingOrder.value = true
  orderResult.value = null
  try {
    const resp = await api.testEcommerceOrder(testOrderNumber.value)
    orderResult.value = resp.data?.data || {}
  } catch (err) {
    showToast(handleHTTPError(err).message, 'destructive')
  } finally {
    testingOrder.value = false
  }
}
</script>
