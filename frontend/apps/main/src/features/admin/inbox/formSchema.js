import * as z from 'zod'
import { isGoDuration, validateEmail } from '@shared-ui/utils/string'
import { AUTH_TYPE_PASSWORD, AUTH_TYPE_OAUTH2 } from '@main/constants/auth.js'

export const createFormSchema = (t) => z.object({
  name: z.string().min(1, t('globals.messages.required')),
  from: z.string().min(1, t('globals.messages.required')),
  reply_to: z
    .string()
    .optional()
    .refine((v) => !v || validateEmail(v), {
      message: t('validation.invalidEmail')
    }),
  enabled: z.boolean().optional(),
  csat_enabled: z.boolean().optional(),
  prompt_tags_on_reply: z.boolean().optional(),
  enable_plus_addressing: z.boolean().optional(),
  auto_assign_on_reply: z.boolean().optional(),
  // T3z: per-inbox skip-PCI-scan toggle. When enabled, the on-ingest
  // pciscrub call is bypassed for incoming messages on this inbox. Useful
  // for inboxes that receive automated payment notifications where card
  // data is already masked by the sender (false-positive expiry hits).
  skip_pci_scan: z.boolean().optional(),
  // MP1: HTML signature template appended to outgoing replies. Supports
  // dynamic placeholders ({{agent.full_name}}, {{inbox.name}}, etc.) which
  // the backend resolves on GET /api/v1/inboxes/{id}/signature.
  signature: z.string().optional(),
  // EC14: comma-separated alias list, e.g. "orders@example.com, support@example.com".
  // Stored as string here to fit the Gmail-style chip input (EmailTagInput);
  // EditInbox/NewInbox split it into a JSON array before posting to the API.
  // Empty = no aliases, only the primary From is available in the reply box.
  aliases: z
    .string()
    .optional()
    .refine((v) => !v || v.split(',').every((e) => {
      const trimmed = e.trim()
      // Allow either "Name <email>" or bare "email" forms.
      const m = trimmed.match(/<([^>]+)>$/)
      return validateEmail(m ? m[1].trim() : trimmed)
    }), {
      message: t('validation.invalidEmail')
    }),
  auth_type: z.enum([AUTH_TYPE_PASSWORD, AUTH_TYPE_OAUTH2]),
  oauth: z.object({
    access_token: z.string().optional(),
    client_id: z.string().optional(),
    client_secret: z.string().optional(),
    expires_at: z.string().optional(),
    provider: z.string().optional(),
    refresh_token: z.string().optional()
  }).optional(),
  imap: z.object({
    host: z.string().min(1, t('globals.messages.required')),
    port: z.number().min(1).max(65535),
    mailbox: z.string().min(1, t('globals.messages.required')),
    username: z.string().min(1, t('globals.messages.required')),
    password: z.string().min(1, t('globals.messages.required')),
    tls_type: z.enum(['none', 'starttls', 'tls']),
    tls_skip_verify: z.boolean().optional(),
    scan_inbox_since: z.string().min(1, t('globals.messages.required')).refine(isGoDuration, {
      message: t('validation.invalidDuration')
    }),
    read_interval: z.string().min(1, t('globals.messages.required')).refine(isGoDuration, {
      message: t('validation.invalidDuration')
    })
  }),
  smtp: z.object({
    host: z.string().min(1, t('globals.messages.required')),
    port: z.number().min(1).max(65535),
    username: z.string().min(1, t('globals.messages.required')),
    password: z.string().min(1, t('globals.messages.required')),
    max_conns: z.number().min(1),
    max_msg_retries: z.number().min(0).max(100),
    idle_timeout: z.string().min(1, t('globals.messages.required')).refine(isGoDuration, {
      message: t('validation.invalidDuration')
    }),
    pool_wait_timeout: z.string().min(1, t('globals.messages.required')).refine(isGoDuration, {
      message: t('validation.invalidDuration')
    }),
    tls_type: z.enum(['none', 'starttls', 'tls']),
    tls_skip_verify: z.boolean().optional(),
    hello_hostname: z.string().optional(),
    auth_protocol: z.enum(['login', 'cram', 'plain', 'none'])
  })
})
