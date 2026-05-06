// Tailwind-derived palette for conversation status pills (FS17). Backend
// stores the key (`status.color`); the frontend resolves the bg/text/label
// here. Centralised so the conversation table view, the conversation header
// pill, and the admin colour picker all share one source of truth.
export const STATUS_COLOR_OPTIONS = [
  { value: 'gray',   label: 'Gray',   bg: '#f3f4f6', text: '#4b5563' },
  { value: 'red',    label: 'Red',    bg: '#fee2e2', text: '#b91c1c' },
  { value: 'orange', label: 'Orange', bg: '#ffedd5', text: '#c2410c' },
  { value: 'amber',  label: 'Amber',  bg: '#fef3c7', text: '#b45309' },
  { value: 'yellow', label: 'Yellow', bg: '#fef9c3', text: '#a16207' },
  { value: 'lime',   label: 'Lime',   bg: '#ecfccb', text: '#4d7c0f' },
  { value: 'green',  label: 'Green',  bg: '#dcfce7', text: '#15803d' },
  { value: 'teal',   label: 'Teal',   bg: '#ccfbf1', text: '#0f766e' },
  { value: 'cyan',   label: 'Cyan',   bg: '#cffafe', text: '#0e7490' },
  { value: 'blue',   label: 'Blue',   bg: '#dbeafe', text: '#1d4ed8' },
  { value: 'indigo', label: 'Indigo', bg: '#e0e7ff', text: '#4338ca' },
  { value: 'purple', label: 'Purple', bg: '#f3e8ff', text: '#7e22ce' },
  { value: 'pink',   label: 'Pink',   bg: '#fce7f3', text: '#be185d' },
  { value: 'rose',   label: 'Rose',   bg: '#ffe4e6', text: '#be123c' },
  { value: 'slate',  label: 'Slate',  bg: '#e2e8f0', text: '#475569' }
]

const COLOR_BY_KEY = Object.fromEntries(STATUS_COLOR_OPTIONS.map(c => [c.value, c]))
const FALLBACK = COLOR_BY_KEY.gray

// Resolve a colour key (or undefined) to a {backgroundColor, color} style
// object suitable for :style binding. Unknown / missing keys fall back to
// gray so old records and unconfigured rows still render.
export function statusColorStyle (colorKey) {
  const c = COLOR_BY_KEY[colorKey] || FALLBACK
  return { backgroundColor: c.bg, color: c.text }
}

// Look up a {value, label, bg, text} entry by key, or the fallback if
// the key isn't recognised.
export function getStatusColor (colorKey) {
  return COLOR_BY_KEY[colorKey] || FALLBACK
}
