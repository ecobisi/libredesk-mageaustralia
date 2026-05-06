import { h } from 'vue'
import { RouterLink } from 'vue-router'
import TeamDataTableDropdown from '@/features/admin/teams/TeamDataTableDropdown.vue'
import { format } from 'date-fns'
import { getI18n } from '@/i18n'

const t = () => getI18n().global.t

export const columns = [
  {
    accessorKey: 'name',
    header: function () {
      return h('div', { class: 'text-center' }, t()('globals.terms.name', 1))
    },
    cell: function ({ row }) {
      const emoji = row.original.emoji
      const color = row.original.color
      const name = row.getValue('name')
      const children = []
      if (emoji) {
        children.push(`${emoji} `)
      } else if (color) {
        children.push(
          h(
            'span',
            {
              class: 'inline-flex h-5 w-5 items-center justify-center rounded text-white font-semibold text-[11px] mr-1 align-middle',
              style: { backgroundColor: color }
            },
            (name || '?').charAt(0).toUpperCase()
          )
        )
      }
      children.push(name)
      return h('div', { class: 'text-center' },
        h(RouterLink,
          {
            to: { name: 'edit-team', params: { id: row.original.id } },
            class: 'text-primary hover:underline'
          },
          () => children
        )
      )
    }
  },
  {
    accessorKey: 'created_at',
    header: function () {
      return h('div', { class: 'text-center' }, t()('globals.terms.createdAt'))
    },
    cell: function ({ row }) {
      return h(
        'div',
        { class: 'text-center' },
        format(row.getValue('created_at'), 'PPpp')
      )
    }
  },
  {
    accessorKey: 'updated_at',
    header: function () {
      return h('div', { class: 'text-center' }, t()('globals.terms.updatedAt'))
    },
    cell: function ({ row }) {
      return h(
        'div',
        { class: 'text-center' },
        format(row.getValue('updated_at'), 'PPpp')
      )
    }
  },
  {
    id: 'actions',
    enableHiding: false,
    enableSorting: false,
    cell: ({ row }) => {
      const team = row.original
      return h(
        'div',
        { class: 'relative' },
        h(TeamDataTableDropdown, {
          team
        })
      )
    }
  }
]
