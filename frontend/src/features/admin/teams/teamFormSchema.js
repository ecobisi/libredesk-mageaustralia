import * as z from 'zod'

export const teamFormSchema = z.object({
  name: z
    .string({
      required_error: 'Team name is required.'
    })
    .min(2, {
      message: 'Team name must be at least 2 characters.'
    }),
  emoji: z.string().optional().nullable().or(z.literal('')),
  color: z.string().regex(/^#[0-9a-fA-F]{6}$/, 'Color must be a hex like #7c3aed.').optional().nullable().or(z.literal('')),
  conversation_assignment_type: z.string({ required_error: 'Conversation assignment type is required.' }),
  max_auto_assigned_conversations: z.coerce.number().optional().default(0),
  timezone: z.string({ required_error: 'Timezone is required.' }),
  business_hours_id: z.number().optional().nullable(),
  sla_policy_id: z.number().optional().nullable(),
})
