<template>
  <form @submit="onSubmit" class="space-y-6 w-full">
    <!-- Basic Fields -->
    <FormField v-if="showFormFields" v-slot="{ componentField }" name="name">
      <FormItem>
        <FormLabel>{{ $t('globals.terms.name') }}</FormLabel>
        <FormControl>
          <Input type="text" placeholder="" v-bind="componentField" />
        </FormControl>
        <FormMessage />
      </FormItem>
    </FormField>

    <FormField v-if="showFormFields" v-slot="{ componentField }" name="from">
      <FormItem>
        <FormLabel>{{ $t('globals.terms.fromEmailAddress') }}</FormLabel>
        <FormControl>
          <Input
            type="text"
            :placeholder="t('admin.inbox.fromEmailAddress.placeholder')"
            v-bind="componentField"
          />
        </FormControl>
        <FormDescription>
          {{ $t('admin.inbox.fromEmailAddress.description') }}
        </FormDescription>
        <FormMessage />
      </FormItem>
    </FormField>

    <FormField v-if="showFormFields" v-slot="{ componentField }" name="reply_to">
      <FormItem>
        <FormLabel>{{ $t('admin.inbox.replyToAddress') }}</FormLabel>
        <FormControl>
          <Input
            type="text"
            :placeholder="t('admin.inbox.replyToAddress.placeholder')"
            v-bind="componentField"
          />
        </FormControl>
        <FormDescription>
          {{ $t('admin.inbox.replyToAddress.description') }}
        </FormDescription>
        <FormMessage />
      </FormItem>
    </FormField>

    <!--
      EC14: From aliases. Each alias is a complete From-style string
      (bare "orders@…" or "Orders <orders@…>"). Reused EmailTagInput keeps
      UX consistent with the reply-box recipient chips agents already know.
      Hidden until showFormFields gates other inbox config fields, same as
      reply_to — we don't want to surface aliases until OAuth/manual setup
      has settled.
    -->
    <FormField v-if="showFormFields" v-slot="{ componentField }" name="aliases">
      <FormItem>
        <FormLabel>{{ $t('admin.inbox.aliases') }}</FormLabel>
        <FormControl>
          <EmailTagInput
            :model-value="componentField.modelValue || ''"
            @update:model-value="componentField['onUpdate:modelValue']"
            :placeholder="$t('admin.inbox.aliases.placeholder')"
          />
        </FormControl>
        <FormDescription>
          {{ $t('admin.inbox.aliases.description') }}
        </FormDescription>
        <FormMessage />
      </FormItem>
    </FormField>

    <!-- Toggle Fields -->
    <FormField v-if="showFormFields" v-slot="{ componentField, handleChange }" name="enabled">
      <FormItem>
        <SwitchField
          :title="$t('globals.terms.enabled')"
          :description="$t('admin.inbox.enabled.description')"
          :checked="componentField.modelValue"
          @update:checked="handleChange"
        />
      </FormItem>
    </FormField>

    <FormField v-if="showFormFields" v-slot="{ componentField, handleChange }" name="csat_enabled">
      <FormItem>
        <SwitchField
          :title="$t('admin.inbox.csatSurveys')"
          :description="$t('admin.inbox.csatSurveys.description_1')"
          :checked="componentField.modelValue"
          @update:checked="handleChange"
        />
      </FormItem>
      <p class="!mt-2 text-muted-foreground text-xs flex items-start gap-1.5">
        <Lightbulb class="size-4" />
        <span>{{ $t('admin.inbox.csatSurveys.description_2') }} {{ $t('admin.inbox.csatSurveys.description_3') }}</span>
      </p>
    </FormField>

    <FormField
      v-if="showFormFields"
      v-slot="{ componentField, handleChange }"
      name="enable_plus_addressing"
    >
      <FormItem>
        <SwitchField
          :title="$t('admin.inbox.enablePlusAddressing')"
          :description="$t('admin.inbox.enablePlusAddressing.description')"
          :checked="componentField.modelValue"
          :disabled="isMicrosoftInbox && componentField.modelValue"
          @update:checked="handleChange"
        />
        <p
          v-if="isMicrosoftInbox"
          class="!mt-2 text-destructive text-xs flex items-start gap-1.5"
        >
          <Lightbulb class="size-4" />
          <span>{{ $t('admin.inbox.enablePlusAddressing.requiredForMicrosoft') }}</span>
        </p>
      </FormItem>
    </FormField>

    <FormField
      v-if="showFormFields"
      v-slot="{ componentField, handleChange }"
      name="prompt_tags_on_reply"
    >
      <FormItem>
        <SwitchField
          :title="$t('admin.inbox.promptTagsOnReply')"
          :description="$t('admin.inbox.promptTagsOnReply.description')"
          :checked="componentField.modelValue"
          @update:checked="handleChange"
        />
      </FormItem>
    </FormField>

    <FormField
      v-if="showFormFields"
      v-slot="{ componentField, handleChange }"
      name="auto_assign_on_reply"
    >
      <FormItem>
        <SwitchField
          :title="$t('admin.inbox.autoAssignOnReply')"
          :description="$t('admin.inbox.autoAssignOnReply.description')"
          :checked="componentField.modelValue"
          @update:checked="handleChange"
        />
      </FormItem>
    </FormField>

    <FormField
      v-if="showFormFields"
      v-slot="{ componentField, handleChange }"
      name="prompt_tags_on_reply"
    >
      <FormItem>
        <SwitchField
          :title="$t('admin.inbox.promptTagsOnReply')"
          :description="$t('admin.inbox.promptTagsOnReply.description')"
          :checked="componentField.modelValue"
          @update:checked="handleChange"
        />
      </FormItem>
    </FormField>

    <!--
      T3z: Skip PCI scanning for this inbox. Bypasses the on-ingest
      pciscrub call so automated payment notifications (where card data
      is already masked by the sender) don't produce false-positive
      flags from plaintext expiry-date matches.
    -->
    <FormField
      v-if="showFormFields"
      v-slot="{ componentField, handleChange }"
      name="skip_pci_scan"
    >
      <FormItem>
        <SwitchField
          :title="$t('admin.inbox.skipPCIScan')"
          :description="$t('admin.inbox.skipPCIScan.description')"
          :checked="componentField.modelValue"
          @update:checked="handleChange"
        />
      </FormItem>
    </FormField>

    <!--
      MP1: Per-inbox HTML signature with placeholder buttons. Stored as a
      raw template; the backend's GET /api/v1/inboxes/{id}/signature
      resolves placeholders with the calling agent's identity (and the
      conversation's contact when a UUID is supplied) and returns ready-
      to-insert HTML. Buttons are quick-inserts — admins can also type the
      placeholders manually if they remember the syntax.
    -->
    <FormField v-if="showFormFields" v-slot="{ componentField }" name="signature">
      <FormItem class="box p-4">
        <div class="space-y-2">
          <FormLabel class="text-base">{{ $t('admin.inbox.signature') }}</FormLabel>
          <FormDescription>
            {{ $t('admin.inbox.signature.description') }}
          </FormDescription>
          <div class="flex flex-wrap gap-1 mb-2">
            <Button type="button" variant="outline" size="xs" @click="insertSignaturePlaceholder('{{agent.first_name}}')">
              {{ $t('admin.inbox.signature.placeholder.agentFirstName') }}
            </Button>
            <Button type="button" variant="outline" size="xs" @click="insertSignaturePlaceholder('{{agent.last_name}}')">
              {{ $t('admin.inbox.signature.placeholder.agentLastName') }}
            </Button>
            <Button type="button" variant="outline" size="xs" @click="insertSignaturePlaceholder('{{agent.full_name}}')">
              {{ $t('admin.inbox.signature.placeholder.agentFullName') }}
            </Button>
            <Button type="button" variant="outline" size="xs" @click="insertSignaturePlaceholder('{{agent.email}}')">
              {{ $t('admin.inbox.signature.placeholder.agentEmail') }}
            </Button>
            <Button type="button" variant="outline" size="xs" @click="insertSignaturePlaceholder('{{inbox.name}}')">
              {{ $t('admin.inbox.signature.placeholder.inboxName') }}
            </Button>
            <Button type="button" variant="outline" size="xs" @click="insertSignaturePlaceholder('{{customer.first_name}}')">
              {{ $t('admin.inbox.signature.placeholder.customerFirstName') }}
            </Button>
          </div>
          <FormControl>
            <Textarea
              v-model="componentField.modelValue"
              @update:modelValue="componentField['onUpdate:modelValue']"
              :placeholder="$t('admin.inbox.signature.placeholder.example')"
              class="min-h-[120px] font-mono text-sm"
            />
          </FormControl>
        </div>
      </FormItem>
    </FormField>

    <FormField v-if="setupMethod" v-slot="{ componentField }" name="auth_type">
      <FormItem>
        <FormControl>
          <Input
            type="hidden"
            :value="setupMethod === 'manual' ? AUTH_TYPE_PASSWORD : AUTH_TYPE_OAUTH2"
            v-bind="componentField"
          />
        </FormControl>
      </FormItem>
    </FormField>

    <!-- Setup Method Selection -->
    <div v-show="!isOAuthInbox && setupMethod === null" class="space-y-4">
      <div class="space-y-2">
        <h3 class="font-semibold text-lg">{{ $t('admin.inbox.oauth.chooseSetupMethod') }}</h3>
        <p class="text-sm text-muted-foreground">
          {{ $t('admin.inbox.oauth.selectConnectionMethod') }}
        </p>
      </div>

      <div class="flex flex-wrap gap-2">
        <MenuCard
          class="shrink-0 w-92 max-w-none"
          :title="$t('globals.terms.google')"
          :subTitle="$t('admin.inbox.oauth.googleDescription')"
          icon="/images/google-logo.svg"
          @click="connectWithGoogle()"
        />
        <MenuCard
          class="shrink-0 w-92 max-w-none"
          :title="$t('globals.terms.microsoft')"
          :subTitle="$t('admin.inbox.oauth.microsoftDescription')"
          icon="/images/microsoft-logo.svg"
          @click="connectWithMicrosoft()"
        />
        <MenuCard
          class="shrink-0 w-92 max-w-none"
          :title="$t('admin.inbox.oauth.otherProvider')"
          :subTitle="$t('admin.inbox.oauth.otherProviderDescription')"
          :icon="Mail"
          @click="setupMethod = 'manual'"
        />
      </div>
    </div>

    <!-- OAuth Connected Status -->
    <div
      v-show="isOAuthInbox"
      class="box p-4 bg-green-50 dark:bg-green-950/20 border-green-200 dark:border-green-800"
    >
      <div class="flex items-start justify-between">
        <div class="flex items-center space-x-3 flex-1">
          <CheckCircle2 class="w-5 h-5 text-green-600 flex-shrink-0" />
          <div class="flex-1">
            <p class="font-semibold text-green-900 dark:text-green-100">
              {{ $t('admin.inbox.oauth.connectedVia', { provider: oauthProvider }) }}
            </p>
            <p class="text-sm text-green-700 dark:text-green-300">{{ oauthEmail }}</p>
            <p
              v-show="oauthClientId"
              class="text-xs text-green-600 dark:text-green-400 font-mono mt-1"
            >
              {{ $t('globals.terms.clientID') }}: {{ oauthClientId.substring(0, 20) }}...{{
                oauthClientId.slice(-8)
              }}
            </p>
          </div>
        </div>

        <Button
          type="button"
          variant="outline"
          size="sm"
          @click="reconnectOAuth"
          :disabled="isSubmittingOAuth"
          class="ml-2 flex-shrink-0"
        >
          <RefreshCw class="w-4 h-4 mr-1" />
          {{ $t('globals.terms.reconnect') }}
        </Button>
      </div>
    </div>

    <!-- OAuth IMAP Configuration -->
    <div v-show="isOAuthInbox" class="box p-4 space-y-4">
      <h3 class="font-semibold">{{ $t('admin.inbox.imapConfig') }}</h3>

      <FormField v-slot="{ componentField }" name="imap.host">
        <FormItem>
          <FormLabel>{{ $t('globals.terms.host') }}</FormLabel>
          <FormControl>
            <Input type="text" placeholder="imap.gmail.com" v-bind="componentField" />
          </FormControl>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="imap.port">
        <FormItem>
          <FormLabel>{{ $t('globals.terms.port') }}</FormLabel>
          <FormControl>
            <Input type="number" placeholder="993" v-bind="componentField" />
          </FormControl>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="imap.tls_type">
        <FormItem>
          <FormLabel>{{ $t('globals.terms.tls') }}</FormLabel>
          <FormControl>
            <Select v-bind="componentField">
              <SelectTrigger>
                <SelectValue :placeholder="t('globals.messages.selectTLS')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="none">{{ $t('globals.terms.off') }}</SelectItem>
                <SelectItem value="tls">SSL/TLS</SelectItem>
                <SelectItem value="starttls">STARTTLS</SelectItem>
              </SelectContent>
            </Select>
          </FormControl>
          <FormDescription>{{ $t('admin.inbox.imap.tls.description') }}</FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="imap.mailbox">
        <FormItem>
          <FormLabel>{{ $t('admin.inbox.mailbox') }}</FormLabel>
          <FormControl>
            <Input type="text" placeholder="INBOX, [Gmail]/Spam" v-bind="componentField" />
          </FormControl>
          <FormDescription>
            {{ $t('admin.inbox.mailbox.description') }}
          </FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="imap.read_interval">
        <FormItem>
          <FormLabel>{{ $t('admin.inbox.imapScanInterval') }}</FormLabel>
          <FormControl>
            <Input type="text" placeholder="1m" v-bind="componentField" />
          </FormControl>
          <FormDescription>
            {{ $t('admin.inbox.imapScanInterval.description') }}
          </FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="imap.scan_inbox_since">
        <FormItem>
          <FormLabel>{{ $t('admin.inbox.imapScanInboxSince') }}</FormLabel>
          <FormControl>
            <Input type="text" placeholder="48h" v-bind="componentField" />
          </FormControl>
          <FormDescription>
            {{ $t('admin.inbox.imapScanInboxSince.description') }}
          </FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>
    </div>

    <!-- OAuth SMTP Configuration -->
    <div v-show="isOAuthInbox" class="box p-4 space-y-4">
      <h3 class="font-semibold">{{ $t('admin.inbox.smtpConfig') }}</h3>

      <FormField v-slot="{ componentField }" name="smtp.host">
        <FormItem>
          <FormLabel>{{ $t('globals.terms.host') }}</FormLabel>
          <FormControl>
            <Input type="text" placeholder="smtp.gmail.com" v-bind="componentField" />
          </FormControl>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="smtp.port">
        <FormItem>
          <FormLabel>{{ $t('globals.terms.port') }}</FormLabel>
          <FormControl>
            <Input type="number" placeholder="587" v-bind="componentField" />
          </FormControl>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="smtp.tls_type">
        <FormItem>
          <FormLabel>{{ t('globals.terms.tls') }}</FormLabel>
          <FormControl>
            <Select v-bind="componentField">
              <SelectTrigger>
                <SelectValue :placeholder="t('globals.messages.selectTLS')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="none">{{ $t('globals.terms.off') }}</SelectItem>
                <SelectItem value="tls">SSL/TLS</SelectItem>
                <SelectItem value="starttls">STARTTLS</SelectItem>
              </SelectContent>
            </Select>
          </FormControl>
          <FormDescription>{{ $t('admin.inbox.tls.description') }}</FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="smtp.max_conns">
        <FormItem>
          <FormLabel>{{ $t('admin.inbox.maxConnections') }}</FormLabel>
          <FormControl>
            <Input type="number" placeholder="10" v-bind="componentField" />
          </FormControl>
          <FormDescription>
            {{ $t('admin.inbox.maxConnections.description') }}
          </FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="smtp.max_msg_retries">
        <FormItem>
          <FormLabel>{{ $t('admin.inbox.maxRetries') }}</FormLabel>
          <FormControl>
            <Input type="number" placeholder="3" v-bind="componentField" />
          </FormControl>
          <FormDescription>{{ $t('admin.inbox.maxRetries.description') }}</FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="smtp.idle_timeout">
        <FormItem>
          <FormLabel>{{ $t('admin.inbox.idleTimeout') }}</FormLabel>
          <FormControl>
            <Input type="text" placeholder="25s" v-bind="componentField" />
          </FormControl>
          <FormDescription>
            {{ $t('admin.inbox.idleTimeout.description') }}
          </FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="smtp.pool_wait_timeout">
        <FormItem>
          <FormLabel>{{ $t('admin.inbox.waitTimeout') }}</FormLabel>
          <FormControl>
            <Input type="text" placeholder="60s" v-bind="componentField" />
          </FormControl>
          <FormDescription>
            {{ $t('admin.inbox.waitTimeout.description') }}
          </FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>
    </div>

    <!-- IMAP Section -->
    <div v-show="!isOAuthInbox && setupMethod === 'manual'" class="box p-4 space-y-4">
      <h3 class="font-semibold">{{ $t('admin.inbox.imapConfig') }}</h3>

      <FormField v-slot="{ componentField }" name="imap.host">
        <FormItem>
          <FormLabel>{{ $t('globals.terms.host') }}</FormLabel>
          <FormControl>
            <Input type="text" placeholder="imap.gmail.com" v-bind="componentField" />
          </FormControl>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="imap.port">
        <FormItem>
          <FormLabel>{{ $t('globals.terms.port') }}</FormLabel>
          <FormControl>
            <Input type="number" placeholder="993" v-bind="componentField" />
          </FormControl>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="imap.mailbox">
        <FormItem>
          <FormLabel>{{ $t('admin.inbox.mailbox') }}</FormLabel>
          <FormControl>
            <Input type="text" placeholder="INBOX, [Gmail]/Spam" v-bind="componentField" />
          </FormControl>
          <FormDescription>
            {{ $t('admin.inbox.mailbox.description') }}
          </FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="imap.username">
        <FormItem>
          <FormLabel>{{ $t('globals.terms.username') }}</FormLabel>
          <FormControl>
            <Input type="text" placeholder="inbox@example.com" v-bind="componentField" />
          </FormControl>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="imap.password">
        <FormItem>
          <FormLabel>{{ $t('globals.terms.password') }}</FormLabel>
          <FormControl>
            <Input type="password" placeholder="••••••••" v-bind="componentField" />
          </FormControl>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="imap.tls_type">
        <FormItem>
          <FormLabel>{{ $t('globals.terms.tls') }}</FormLabel>
          <FormControl>
            <Select v-bind="componentField">
              <SelectTrigger>
                <SelectValue :placeholder="t('globals.messages.selectTLS')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="none">{{ $t('globals.terms.off') }}</SelectItem>
                <SelectItem value="tls">SSL/TLS</SelectItem>
                <SelectItem value="starttls">STARTTLS</SelectItem>
              </SelectContent>
            </Select>
          </FormControl>
          <FormDescription>{{ $t('admin.inbox.imap.tls.description') }}</FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="imap.read_interval">
        <FormItem>
          <FormLabel>{{ $t('admin.inbox.imapScanInterval') }}</FormLabel>
          <FormControl>
            <Input type="text" placeholder="5m" v-bind="componentField" />
          </FormControl>
          <FormDescription>
            {{ $t('admin.inbox.imapScanInterval.description') }}
          </FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="imap.scan_inbox_since">
        <FormItem>
          <FormLabel>{{ $t('admin.inbox.imapScanInboxSince') }}</FormLabel>
          <FormControl>
            <Input type="text" placeholder="48h" v-bind="componentField" />
          </FormControl>
          <FormDescription>
            {{ $t('admin.inbox.imapScanInboxSince.description') }}
          </FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField, handleChange }" name="imap.tls_skip_verify">
        <FormItem>
          <SwitchField
            :title="$t('admin.inbox.skipTLSVerification')"
            :description="$t('admin.inbox.skipTLSVerification.description')"
            :checked="componentField.modelValue"
            @update:checked="handleChange"
          />
        </FormItem>
      </FormField>
    </div>

    <!-- SMTP Section -->
    <div v-show="!isOAuthInbox && setupMethod === 'manual'" class="box p-4 space-y-4">
      <h3 class="font-semibold">{{ $t('admin.inbox.smtpConfig') }}</h3>

      <FormField v-slot="{ componentField }" name="smtp.host">
        <FormItem>
          <FormLabel>{{ $t('globals.terms.host') }}</FormLabel>
          <FormControl>
            <Input type="text" placeholder="smtp.gmail.com" v-bind="componentField" />
          </FormControl>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="smtp.port">
        <FormItem>
          <FormLabel>{{ $t('globals.terms.port') }}</FormLabel>
          <FormControl>
            <Input type="number" placeholder="587" v-bind="componentField" />
          </FormControl>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="smtp.username">
        <FormItem>
          <FormLabel>{{ $t('globals.terms.username') }}</FormLabel>
          <FormControl>
            <Input type="text" placeholder="user@example.com" v-bind="componentField" />
          </FormControl>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="smtp.password">
        <FormItem>
          <FormLabel>{{ $t('globals.terms.password') }}</FormLabel>
          <FormControl>
            <Input type="password" placeholder="••••••••" v-bind="componentField" />
          </FormControl>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="smtp.max_conns">
        <FormItem>
          <FormLabel>{{ $t('admin.inbox.maxConnections') }}</FormLabel>
          <FormControl>
            <Input type="number" placeholder="10" v-bind="componentField" />
          </FormControl>
          <FormDescription>
            {{ $t('admin.inbox.maxConnections.description') }}
          </FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="smtp.max_msg_retries">
        <FormItem>
          <FormLabel>{{ $t('admin.inbox.maxRetries') }}</FormLabel>
          <FormControl>
            <Input type="number" placeholder="3" v-bind="componentField" />
          </FormControl>
          <FormDescription>{{ $t('admin.inbox.maxRetries.description') }} </FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="smtp.idle_timeout">
        <FormItem>
          <FormLabel>{{ $t('admin.inbox.idleTimeout') }}</FormLabel>
          <FormControl>
            <Input type="text" placeholder="25s" v-bind="componentField" />
          </FormControl>
          <FormDescription>
            {{ $t('admin.inbox.idleTimeout.description') }}
          </FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="smtp.pool_wait_timeout">
        <FormItem>
          <FormLabel>{{ $t('admin.inbox.waitTimeout') }}</FormLabel>
          <FormControl>
            <Input type="text" placeholder="60s" v-bind="componentField" />
          </FormControl>
          <FormDescription>
            {{ $t('admin.inbox.waitTimeout.description') }}
          </FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="smtp.auth_protocol">
        <FormItem>
          <FormLabel>{{ $t('admin.inbox.authProtocol') }}</FormLabel>
          <FormControl>
            <Select v-bind="componentField">
              <SelectTrigger>
                <SelectValue :placeholder="t('placeholders.selectProtocol')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="login">{{ $t('admin.inbox.authProtocol.login') }}</SelectItem>
                <SelectItem value="cram">CRAM</SelectItem>
                <SelectItem value="plain">{{ $t('admin.inbox.authProtocol.plain') }}</SelectItem>
                <SelectItem value="none">{{ $t('globals.terms.none') }}</SelectItem>
              </SelectContent>
            </Select>
          </FormControl>
          <FormDescription> {{ $t('admin.inbox.authProtocol.description') }} </FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="smtp.tls_type">
        <FormItem>
          <FormLabel>{{ $t('globals.terms.tls') }}</FormLabel>
          <FormControl>
            <Select v-bind="componentField">
              <SelectTrigger>
                <SelectValue :placeholder="t('globals.messages.selectTLS')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="none">{{ $t('globals.terms.off') }}</SelectItem>
                <SelectItem value="tls">SSL/TLS</SelectItem>
                <SelectItem value="starttls">STARTTLS</SelectItem>
              </SelectContent>
            </Select>
          </FormControl>
          <FormDescription> {{ $t('admin.inbox.tls.description') }} </FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField }" name="smtp.hello_hostname">
        <FormItem>
          <FormLabel>{{ $t('admin.inbox.heloHostname') }}</FormLabel>
          <FormControl>
            <Input type="text" placeholder="" v-bind="componentField" />
          </FormControl>
          <FormDescription>
            {{ $t('admin.inbox.heloHostname.description') }}
          </FormDescription>
          <FormMessage />
        </FormItem>
      </FormField>

      <FormField v-slot="{ componentField, handleChange }" name="smtp.tls_skip_verify">
        <FormItem>
          <SwitchField
            :title="$t('admin.inbox.skipTLSVerification')"
            :description="$t('admin.inbox.skipTLSVerification.description')"
            :checked="componentField.modelValue"
            @update:checked="handleChange"
          />
        </FormItem>
      </FormField>
    </div>

    <!-- Test Connection Section -->
    <div v-show="setupMethod === 'manual' || isOAuthInbox" class="border-t pt-6 mt-6 space-y-6">
      <h3 class="text-base font-semibold">{{ $t('admin.inbox.testConnection') }}</h3>

      <!-- IMAP Test -->
      <div class="space-y-3">
        <div class="flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            @click="runIMAPTest"
            :disabled="isTestingIMAP"
          >
            <Loader2 v-if="isTestingIMAP" class="w-4 h-4 mr-2 animate-spin" />
            {{ isTestingIMAP ? $t('admin.inbox.testConnection.testing') : $t('admin.inbox.testConnection.testIMAP') }}
          </Button>
        </div>
        <div v-if="imapTestLogs.length > 0" class="space-y-2">
          <Label>{{ $t('admin.inbox.imapLog') }}</Label>
          <div
            class="bg-muted p-3 rounded-md font-mono text-xs max-h-48 overflow-y-auto"
            :class="imapTestSuccess === true ? 'border-green-500 border' : imapTestSuccess === false ? 'border-red-500 border' : ''"
          >
            <div v-for="(log, index) in imapTestLogs" :key="index" class="py-0.5">{{ log }}</div>
          </div>
        </div>
      </div>

      <!-- SMTP Test -->
      <div class="space-y-3">
        <div class="flex items-center gap-2">
          <Input
            v-model="smtpTestEmail"
            type="email"
            :placeholder="$t('admin.inbox.testConnection.emailPlaceholder')"
            class="flex-1 max-w-xs"
          />
          <Button
            type="button"
            variant="outline"
            @click="runSMTPTest"
            :disabled="isTestingSMTP"
          >
            <Loader2 v-if="isTestingSMTP" class="w-4 h-4 mr-2 animate-spin" />
            {{ isTestingSMTP ? $t('admin.inbox.testConnection.testing') : $t('admin.inbox.testConnection.testSMTP') }}
          </Button>
        </div>
        <div v-if="smtpTestLogs.length > 0" class="space-y-2">
          <Label>{{ $t('admin.inbox.smtpLog') }}</Label>
          <div
            class="bg-muted p-3 rounded-md font-mono text-xs max-h-48 overflow-y-auto"
            :class="smtpTestSuccess === true ? 'border-green-500 border' : smtpTestSuccess === false ? 'border-red-500 border' : ''"
          >
            <div v-for="(log, index) in smtpTestLogs" :key="index" class="py-0.5">{{ log }}</div>
          </div>
        </div>
      </div>
    </div>

    <Button type="submit" :is-loading="isLoading" :disabled="isLoading">
      {{ submitLabel }}
    </Button>
  </form>

  <!-- OAuth Credentials Modal -->
  <Dialog v-model:open="showOAuthModal">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>
          {{
            flowType === 'reconnect'
              ? $t('admin.inbox.oauth.reconnectAccount', {
                  provider:
                    selectedProvider === PROVIDER_GOOGLE
                      ? $t('globals.terms.google')
                      : $t('globals.terms.microsoft')
                })
              : $t('admin.inbox.oauth.connectAccount', {
                  provider:
                    selectedProvider === PROVIDER_GOOGLE
                      ? $t('globals.terms.google')
                      : $t('globals.terms.microsoft')
                })
          }}
        </DialogTitle>
        <DialogDescription>
          {{
            flowType === 'reconnect'
              ? $t('admin.inbox.oauth.reconnectDescription')
              : $t('admin.inbox.oauth.followSteps')
          }}
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4">
        <div v-if="flowType === 'new_inbox'" class="space-y-4">
          <p class="text-sm">
            {{ $t('admin.inbox.oauth.step1CreateApp') }}
            <a
              :href="
                selectedProvider === PROVIDER_GOOGLE
                  ? 'https://console.cloud.google.com/apis/credentials'
                  : 'https://entra.microsoft.com/'
              "
              target="_blank"
              class="text-primary underline"
            >
              {{
                selectedProvider === PROVIDER_GOOGLE
                  ? $t('admin.inbox.oauth.googleCloudConsole')
                  : $t('admin.inbox.oauth.microsoftAzurePortal')
              }}
            </a>
          </p>

          <div class="space-y-1">
            <p class="text-sm">{{ $t('admin.inbox.oauth.step2AddCallback') }}</p>
            <div class="flex items-center gap-2">
              <Input :model-value="callbackUrl" readonly class="font-mono text-xs" />
              <Button
                type="button"
                variant="outline"
                size="sm"
                @click="copyToClipboard(callbackUrl)"
              >
                {{ $t('globals.terms.copy') }}
              </Button>
            </div>
          </div>

          <p class="text-sm">{{ $t('admin.inbox.oauth.step3EnterCredentials') }}</p>
        </div>

        <div class="space-y-2">
          <label class="text-sm font-medium">{{ $t('globals.terms.clientID') }}</label>
          <Input
            v-model="oauthCredentials.client_id"
            :placeholder="t('admin.inbox.oauth.enterClientID')"
            :disabled="isSubmittingOAuth"
          />
        </div>

        <div class="space-y-2">
          <label class="text-sm font-medium">{{ $t('globals.terms.clientSecret') }}</label>
          <Input
            v-model="oauthCredentials.client_secret"
            type="password"
            :placeholder="t('admin.inbox.oauth.enterClientSecret')"
            :disabled="isSubmittingOAuth"
          />
        </div>

        <div v-if="selectedProvider === PROVIDER_MICROSOFT" class="space-y-2">
          <label class="text-sm font-medium">{{ $t('globals.terms.tenantID') }}</label>
          <Input v-model="oauthCredentials.tenant_id" :disabled="isSubmittingOAuth" />
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="showOAuthModal = false" :disabled="isSubmittingOAuth">
          {{ $t('globals.messages.cancel') }}
        </Button>
        <Button @click="submitOAuthCredentials" :disabled="isSubmittingOAuth">
          {{ isSubmittingOAuth ? $t('globals.messages.connecting') : $t('globals.terms.continue') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<script setup>
import { watch, computed, ref } from 'vue'
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'
import { createFormSchema } from './formSchema.js'
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
  FormDescription
} from '@shared-ui/components/ui/form/index.js'
import { Label } from '@shared-ui/components/ui/label'
import { Input } from '@shared-ui/components/ui/input/index.js'
import { Textarea } from '@shared-ui/components/ui/textarea'
import SwitchField from '@shared-ui/components/SwitchField.vue'
import { Button } from '@shared-ui/components/ui/button/index.js'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@shared-ui/components/ui/select/index.js'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@shared-ui/components/ui/dialog'
import { CheckCircle2, Loader2, Lightbulb, Mail, RefreshCw } from 'lucide-vue-next'
import MenuCard from '@main/components/layout/MenuCard.vue'
import EmailTagInput from '@main/components/EmailTagInput.vue'
import { useI18n } from 'vue-i18n'
import api from '@/api'
import { useEmitter } from '@/composables/useEmitter'
import { EMITTER_EVENTS } from '@/constants/emitterEvents.js'
import {
  AUTH_TYPE_PASSWORD,
  AUTH_TYPE_OAUTH2,
  PROVIDER_GOOGLE,
  PROVIDER_MICROSOFT
} from '@/constants/auth.js'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import { useAppSettingsStore } from '@/stores/appSettings'

const props = defineProps({
  initialValues: {
    type: Object,
    default: () => ({})
  },
  submitForm: {
    type: Function,
    required: true
  },
  submitLabel: {
    type: String,
    default: ''
  },
  isNewForm: {
    type: Boolean,
    default: false
  },
  isLoading: {
    type: Boolean,
    default: false
  }
})

const { t } = useI18n()
const emitter = useEmitter()
const appSettingsStore = useAppSettingsStore()

// OAuth detection
const isOAuthInbox = ref(false)

// Setup method selection: null | PROVIDER_GOOGLE | PROVIDER_MICROSOFT | 'manual'
const setupMethod = ref(null)

// OAuth modal state
const showOAuthModal = ref(false)
const selectedProvider = ref('')
const flowType = ref('new_inbox') // "new_inbox" or "reconnect"
const oauthCredentials = ref({
  client_id: '',
  client_secret: '',
  tenant_id: ''
})
const isSubmittingOAuth = ref(false)

// Computed callback URL for OAuth
const callbackUrl = computed(() => {
  const rootUrl = appSettingsStore.settings['app.root_url']
  return `${rootUrl}/api/v1/inboxes/oauth/${selectedProvider.value}/callback`
})

// Show form fields when OAuth is connected or manual setup is selected
const showFormFields = computed(
  () =>
    isOAuthInbox.value ||
    setupMethod.value === 'manual' ||
    (props.initialValues?.imap && Object.keys(props.initialValues?.imap).length > 0)
)

// MP1: append a placeholder token to the signature template. Append (rather
// than splice at cursor) keeps the helper trivially predictable — admins can
// reorder by hand after clicking. Mirrors v1.0.3's behaviour.
const insertSignaturePlaceholder = (placeholder) => {
  const current = form.values.signature || ''
  form.setFieldValue('signature', current + placeholder)
}

const form = useForm({
  validationSchema: computed(() => toTypedSchema(createFormSchema(t))),
  initialValues: {
    name: '',
    from: '',
    reply_to: '',
    aliases: '',
    enabled: true,
    csat_enabled: false,
    prompt_tags_on_reply: false,
    enable_plus_addressing: true,
    auto_assign_on_reply: false,
    skip_pci_scan: false,
    signature: '',
    auth_type: AUTH_TYPE_PASSWORD,
    imap: {
      host: 'imap.gmail.com',
      port: 993,
      mailbox: 'INBOX',
      username: '',
      password: '',
      tls_type: 'none',
      read_interval: '5m',
      scan_inbox_since: '48h',
      tls_skip_verify: false
    },
    smtp: {
      host: 'smtp.gmail.com',
      port: 587,
      username: '',
      password: '',
      max_conns: 10,
      max_msg_retries: 3,
      idle_timeout: '25s',
      pool_wait_timeout: '120s',
      auth_protocol: 'login',
      tls_type: 'none',
      hello_hostname: '',
      tls_skip_verify: false
    }
  }
})

// OAuth computed properties
const oauthProvider = computed(() => {
  const provider = form.values.oauth?.provider
  return provider ? provider.charAt(0).toUpperCase() + provider.slice(1) : 'Unknown'
})

const oauthEmail = computed(() => {
  return form.values.imap?.username || form.values.smtp?.username || ''
})

const oauthClientId = computed(() => {
  return form.values.oauth?.client_id || ''
})

const isMicrosoftInbox = computed(() => form.values.oauth?.provider === PROVIDER_MICROSOFT)

const submitLabel = computed(() => {
  return (
    props.submitLabel ||
    (props.isNewForm ? t('globals.messages.create') : t('globals.messages.save'))
  )
})

const onSubmit = form.handleSubmit(async (values) => {
  await props.submitForm(values)
})

const connectWithGoogle = () => {
  flowType.value = 'new_inbox'
  selectedProvider.value = PROVIDER_GOOGLE
  showOAuthModal.value = true
}

const connectWithMicrosoft = () => {
  flowType.value = 'new_inbox'
  selectedProvider.value = PROVIDER_MICROSOFT
  showOAuthModal.value = true
}

const reconnectOAuth = () => {
  const provider = form.values.oauth?.provider
  const clientId = form.values.oauth?.client_id
  const tenantId = form.values.oauth?.tenant_id

  if (!provider) return

  // Set flow type to reconnect
  flowType.value = 'reconnect'

  // Set provider and pre-fill credentials
  selectedProvider.value = provider
  oauthCredentials.value.client_id = clientId || ''
  oauthCredentials.value.client_secret = '' // Always require user to re-enter secret
  oauthCredentials.value.tenant_id = tenantId || ''

  // Show modal for user to edit credentials
  showOAuthModal.value = true
}

const submitOAuthCredentials = async () => {
  if (!oauthCredentials.value.client_id || !oauthCredentials.value.client_secret) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: t('admin.inbox.oauth.clientIDSecretRequired')
    })
    return
  }

  try {
    isSubmittingOAuth.value = true
    const payload = {
      ...oauthCredentials.value,
      flow_type: flowType.value
    }

    // Include inbox_id for reconnect flow (props.initialValues.id exists in edit mode)
    if (flowType.value === 'reconnect' && props.initialValues?.id) {
      payload.inbox_id = props.initialValues.id
    }

    const response = await api.initiateOAuthFlow(selectedProvider.value, payload)
    window.location.href = response.data.data
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  }
}

const copyToClipboard = async (text) => {
  try {
    await navigator.clipboard.writeText(text)
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      description: t('globals.messages.copied')
    })
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: t('globals.messages.somethingWentWrong')
    })
  }
}

// Test connection state
const isTestingIMAP = ref(false)
const isTestingSMTP = ref(false)
const imapTestLogs = ref([])
const smtpTestLogs = ref([])
const imapTestSuccess = ref(null)
const smtpTestSuccess = ref(null)
const smtpTestEmail = ref('')

const runIMAPTest = async () => {
  isTestingIMAP.value = true
  imapTestLogs.value = []
  imapTestSuccess.value = null
  try {
    const values = form.values
    const payload = {
      imap: values.imap,
      auth_type: values.auth_type
    }
    // Pass inbox_id when editing an existing inbox so the backend can substitute
    // the stored password when the frontend has only a masked dummy.
    if (props.initialValues?.id) {
      payload.inbox_id = props.initialValues.id
    }
    const response = await api.testInboxConnection(payload)
    imapTestLogs.value = response.data.data.imap_logs || []
    imapTestSuccess.value = response.data.data.success
  } catch (error) {
    imapTestLogs.value = [handleHTTPError(error).message]
    imapTestSuccess.value = false
  } finally {
    isTestingIMAP.value = false
  }
}

const runSMTPTest = async () => {
  isTestingSMTP.value = true
  smtpTestLogs.value = []
  smtpTestSuccess.value = null
  try {
    const values = form.values
    const payload = {
      smtp: values.smtp,
      auth_type: values.auth_type,
      test_email: smtpTestEmail.value
    }
    // Pass inbox_id when editing an existing inbox so the backend can substitute
    // the stored password when the frontend has only a masked dummy.
    if (props.initialValues?.id) {
      payload.inbox_id = props.initialValues.id
    }
    const response = await api.testInboxConnection(payload)
    smtpTestLogs.value = response.data.data.smtp_logs || []
    smtpTestSuccess.value = response.data.data.success
  } catch (error) {
    smtpTestLogs.value = [handleHTTPError(error).message]
    smtpTestSuccess.value = false
  } finally {
    isTestingSMTP.value = false
  }
}

watch(
  () => props.initialValues,
  (newValues) => {
    if (Object.keys(newValues).length === 0) {
      return
    }
    if (newValues.config?.auth_type === AUTH_TYPE_OAUTH2) {
      isOAuthInbox.value = true
      setupMethod.value = 'oauth'
    } else {
      isOAuthInbox.value = false
      setupMethod.value = 'manual'
    }
    form.setValues(newValues)
  },
  { deep: true, immediate: true }
)
</script>
