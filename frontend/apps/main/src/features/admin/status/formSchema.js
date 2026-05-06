import * as z from 'zod'
import { STATUS_COLOR_OPTIONS } from '@/constants/statusColors'

const COLOR_KEYS = STATUS_COLOR_OPTIONS.map(c => c.value)

export const createFormSchema = (t) => z.object({
  name: z
    .string({
      required_error: t('globals.messages.required'),
    })
    .min(1, {
      message: t('validation.minmax', {
        min: 1,
        max: 25,
      })
    })
    .max(25, {
      message: t('validation.minmax', {
        min: 1,
        max: 25,
      })
    }),
  category: z.enum(['open', 'waiting', 'resolved'], {
    required_error: t('globals.messages.required'),
  }),
  // Color is optional in the form (backend defaults to "gray" when empty).
  // Validating against the known palette keeps a malformed payload from
  // being persisted, but we accept the empty string for the create path
  // before the user picks a colour.
  color: z.union([
    z.literal(''),
    z.enum(COLOR_KEYS)
  ]).optional()
})
