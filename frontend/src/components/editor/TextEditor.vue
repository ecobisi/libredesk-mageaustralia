<template>
  <div class="editor-wrapper h-full overflow-y-auto" :class="{ 'pointer-events-none': disabled }">
    <BubbleMenu
      :editor="editor"
      :tippy-options="{ duration: 100, maxWidth: 'none' }"
      v-if="editor"
      class="bg-background p-1 box will-change-transform"
    >
      <div class="flex space-x-1 items-center">
        <DropdownMenu v-if="aiPrompts.length > 0">
          <DropdownMenuTrigger>
            <Button size="sm" variant="ghost" class="flex items-center justify-center">
              <span class="flex items-center">
                <span class="text-medium">AI</span>
                <Bot size="14" class="ml-1" />
                <ChevronDown class="w-4 h-4 ml-2" />
              </span>
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            <DropdownMenuItem
              v-for="prompt in aiPrompts"
              :key="prompt.key"
              @select="emitPrompt(prompt.key)"
            >
              {{ prompt.title }}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <Button
          size="sm"
          variant="ghost"
          @click.prevent="editor?.chain().focus().toggleBold().run()"
          :class="{ 'bg-gray-200 dark:bg-secondary': editor?.isActive('bold') }"
        >
          <Bold size="14" />
        </Button>
        <Button
          size="sm"
          variant="ghost"
          @click.prevent="editor?.chain().focus().toggleItalic().run()"
          :class="{ 'bg-gray-200 dark:bg-secondary': editor?.isActive('italic') }"
        >
          <Italic size="14" />
        </Button>
        <Button
          size="sm"
          variant="ghost"
          @click.prevent="editor?.chain().focus().toggleUnderline().run()"
          :class="{ 'bg-gray-200 dark:bg-secondary': editor?.isActive('underline') }"
        >
          <UnderlineIcon size="14" />
        </Button>
        <Button
          size="sm"
          variant="ghost"
          @click.prevent="editor?.chain().focus().toggleStrike().run()"
          :class="{ 'bg-gray-200 dark:bg-secondary': editor?.isActive('strike') }"
        >
          <Strikethrough size="14" />
        </Button>
        <!-- Text color -->
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <Button size="sm" variant="ghost" class="flex items-center">
              <Palette size="14" />
              <ChevronDown class="w-3 h-3 ml-0.5" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent class="p-2 w-auto">
            <div class="text-xs text-muted-foreground mb-1">Text color</div>
            <div class="grid grid-cols-8 gap-1">
              <button
                v-for="c in paletteColors"
                :key="'fg-' + c"
                class="w-5 h-5 rounded border border-gray-300 hover:scale-110 transition"
                :style="{ backgroundColor: c }"
                @click.prevent="editor?.chain().focus().setColor(c).run()"
              />
              <button
                class="w-5 h-5 rounded border border-gray-300 text-xs"
                title="Clear"
                @click.prevent="editor?.chain().focus().unsetColor().run()"
              >×</button>
            </div>
          </DropdownMenuContent>
        </DropdownMenu>
        <!-- Highlight color -->
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <Button size="sm" variant="ghost" class="flex items-center">
              <Highlighter size="14" />
              <ChevronDown class="w-3 h-3 ml-0.5" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent class="p-2 w-auto">
            <div class="text-xs text-muted-foreground mb-1">Highlight</div>
            <div class="grid grid-cols-8 gap-1">
              <button
                v-for="c in highlightColors"
                :key="'bg-' + c"
                class="w-5 h-5 rounded border border-gray-300 hover:scale-110 transition"
                :style="{ backgroundColor: c }"
                @click.prevent="editor?.chain().focus().toggleHighlight({ color: c }).run()"
              />
              <button
                class="w-5 h-5 rounded border border-gray-300 text-xs"
                title="Clear"
                @click.prevent="editor?.chain().focus().unsetHighlight().run()"
              >×</button>
            </div>
          </DropdownMenuContent>
        </DropdownMenu>
        <!-- Alignment -->
        <Button
          size="sm"
          variant="ghost"
          @click.prevent="editor?.chain().focus().setTextAlign('left').run()"
          :class="{ 'bg-gray-200 dark:bg-secondary': editor?.isActive({ textAlign: 'left' }) }"
        >
          <AlignLeft size="14" />
        </Button>
        <Button
          size="sm"
          variant="ghost"
          @click.prevent="editor?.chain().focus().setTextAlign('center').run()"
          :class="{ 'bg-gray-200 dark:bg-secondary': editor?.isActive({ textAlign: 'center' }) }"
        >
          <AlignCenter size="14" />
        </Button>
        <Button
          size="sm"
          variant="ghost"
          @click.prevent="editor?.chain().focus().setTextAlign('right').run()"
          :class="{ 'bg-gray-200 dark:bg-secondary': editor?.isActive({ textAlign: 'right' }) }"
        >
          <AlignRight size="14" />
        </Button>
        <Button
          size="sm"
          variant="ghost"
          @click.prevent="editor?.chain().focus().toggleBulletList().run()"
          :class="{ 'bg-gray-200 dark:bg-secondary': editor?.isActive('bulletList') }"
        >
          <List size="14" />
        </Button>
        <Button
          size="sm"
          variant="ghost"
          @click.prevent="editor?.chain().focus().toggleOrderedList().run()"
          :class="{ 'bg-gray-200 dark:bg-secondary': editor?.isActive('orderedList') }"
        >
          <ListOrdered size="14" />
        </Button>
        <Button
          size="sm"
          variant="ghost"
          @click.prevent="openLinkModal"
          :class="{ 'bg-gray-200 dark:bg-secondary': editor?.isActive('link') }"
        >
          <LinkIcon size="14" />
        </Button>
        <!-- Image upload button -->
        <Button
          size="sm"
          variant="ghost"
          @click.prevent="triggerImageUpload"
          :disabled="isUploadingImage"
        >
          <ImageIcon size="14" />
        </Button>
      </div>
    </BubbleMenu>
    <EditorContent :editor="editor" class="native-html" />

    <!-- Hidden file input for image upload -->
    <input
      ref="imageInput"
      type="file"
      accept="image/*"
      class="hidden"
      @change="handleImageSelect"
    />

    <!-- Upload indicator -->
    <div v-if="isUploadingImage" class="text-xs text-muted-foreground mt-1 flex items-center gap-1">
      <Loader2 size="12" class="animate-spin" />
      Uploading image...
    </div>

    <Dialog v-model:open="showLinkDialog">
      <DialogContent class="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle>
            {{
              editor?.isActive('link')
                ? $t('globals.messages.edit', {
                    name:
                      $t('globals.terms.link', 1).toLowerCase() +
                      ' ' +
                      $t('globals.terms.url', 1).toLowerCase()
                  })
                : $t('globals.messages.add', {
                    name:
                      $t('globals.terms.link', 1).toLowerCase() +
                      ' ' +
                      $t('globals.terms.url', 1).toLowerCase()
                  })
            }}
          </DialogTitle>
          <DialogDescription></DialogDescription>
        </DialogHeader>
        <form @submit.stop.prevent="setLink">
          <div class="grid gap-4 py-4">
            <Input
              v-model="linkUrl"
              type="text"
              :placeholder="$t('globals.messages.enter', { name: $t('globals.terms.url', 1) })"
              @keydown.enter.prevent="setLink"
            />
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              @click="unsetLink"
              v-if="editor?.isActive('link')"
            >
              {{ $t('globals.messages.remove', { name: $t('globals.terms.link', 1) }) }}
            </Button>
            <Button type="submit">
              {{ $t('globals.messages.save') }}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { useEditor, EditorContent, BubbleMenu, Extension } from '@tiptap/vue-3'
import {
  ChevronDown,
  Bold,
  Italic,
  Underline as UnderlineIcon,
  Strikethrough,
  Bot,
  List,
  ListOrdered,
  Link as LinkIcon,
  Image as ImageIcon,
  Loader2,
  Palette,
  Highlighter,
  AlignLeft,
  AlignCenter,
  AlignRight
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogDescription
} from '@/components/ui/dialog'
import Placeholder from '@tiptap/extension-placeholder'
import Image from '@tiptap/extension-image'
import StarterKit from '@tiptap/starter-kit'
import Underline from '@tiptap/extension-underline'
import TextStyle from '@tiptap/extension-text-style'
import Color from '@tiptap/extension-color'
import Highlight from '@tiptap/extension-highlight'
import TextAlign from '@tiptap/extension-text-align'
import { liftListItem as pmLiftListItem } from '@tiptap/pm/schema-list'

const ListExitExtension = Extension.create({
  name: 'listExit',
  priority: 1000,
  addKeyboardShortcuts() {
    return {
      Enter: ({ editor }) => {
        const { state, view } = editor
        const { $from, empty } = state.selection
        if (!empty) return false
        const listItemType = state.schema.nodes.listItem
        if (!listItemType) return false
        // Walk up looking for the nearest listItem ancestor.
        let liDepth = -1
        for (let d = $from.depth; d > 0; d--) {
          if ($from.node(d).type === listItemType) { liDepth = d; break }
        }
        if (liDepth === -1) return false
        // The node directly inside the listItem that contains the cursor (usually a paragraph).
        const paraDepth = liDepth + 1
        if (paraDepth > $from.depth) return false
        const para = $from.node(paraDepth)
        if (!para) return false
        // Is the current paragraph empty (no content, or only a trailing hardBreak)?
        const emptyPara = para.content.size === 0 ||
          (para.childCount === 1 && para.firstChild?.type?.name === 'hardBreak')
        if (!emptyPara) return false
        // Only lift if this empty paragraph is the LAST child of the listItem — otherwise there's
        // meaningful content after the cursor inside this item and we should let the default run.
        const li = $from.node(liDepth)
        const idxInLi = $from.index(liDepth)
        if (idxInLi !== li.childCount - 1) return false
        return pmLiftListItem(listItemType)(state, view.dispatch)
      },
    }
  },
})
import Link from '@tiptap/extension-link'
import Mention from '@tiptap/extension-mention'
import Table from '@tiptap/extension-table'
import TableRow from '@tiptap/extension-table-row'
import TableCell from '@tiptap/extension-table-cell'
import TableHeader from '@tiptap/extension-table-header'
import mentionSuggestion from './mentionSuggestion'
import { useEmitter } from '@/composables/useEmitter'
import { EMITTER_EVENTS } from '@/constants/emitterEvents.js'
import { handleHTTPError } from '@/utils/http'
import api from '@/api'

const textContent = defineModel('textContent', { default: '' })
const htmlContent = defineModel('htmlContent', { default: '' })

const paletteColors = [
  '#000000', '#434343', '#666666', '#999999', '#b7b7b7', '#cccccc', '#d9d9d9', '#ffffff',
  '#980000', '#ff0000', '#ff9900', '#ffff00', '#00ff00', '#00ffff', '#4a86e8', '#0000ff',
  '#9900ff', '#ff00ff', '#e6b8af', '#f4cccc', '#fce5cd', '#fff2cc', '#d9ead3', '#d0e0e3',
  '#c9daf8', '#cfe2f3', '#d9d2e9', '#ead1dc'
]
const highlightColors = [
  '#ffff00', '#ffcc00', '#ff9900', '#ff6600', '#ff0000', '#ff00ff', '#9900ff', '#0000ff',
  '#00ffff', '#00ff00', '#99cc00', '#cccccc', '#fce5cd', '#fff2cc', '#d9ead3', '#c9daf8'
]
const showLinkDialog = ref(false)
const linkUrl = ref('')
const imageInput = ref(null)
const isUploadingImage = ref(false)
const emitter = useEmitter()

const props = defineProps({
  placeholder: String,
  insertContent: String,
  autoFocus: {
    type: Boolean,
    default: true
  },
  aiPrompts: {
    type: Array,
    default: () => []
  },
  disabled: {
    type: Boolean,
    default: false
  },
  enableMentions: {
    type: Boolean,
    default: false
  },
  getSuggestions: {
    type: Function,
    default: null
  }
})

const emit = defineEmits(['send', 'aiPromptSelected', 'mentionsChanged', 'filesDropped'])

const emitPrompt = (key) => emit('aiPromptSelected', key)

/**
 * Resize an image file if it exceeds max upload dimensions.
 * Preserves quality — only downsizes truly massive images (>2000px)
 * to avoid uploading 10MB+ files. Display size is controlled in the editor.
 */
const MAX_UPLOAD_DIM = 2000

const resizeImage = (file) => {
  return new Promise((resolve) => {
    if (!file.type.startsWith('image/') || file.type === 'image/gif') {
      resolve(file)
      return
    }

    const img = new window.Image()
    const url = URL.createObjectURL(file)

    img.onload = () => {
      URL.revokeObjectURL(url)

      if (img.width <= MAX_UPLOAD_DIM && img.height <= MAX_UPLOAD_DIM) {
        resolve(file)
        return
      }

      let newWidth = img.width
      let newHeight = img.height

      if (newWidth > MAX_UPLOAD_DIM) {
        newHeight = Math.round(newHeight * (MAX_UPLOAD_DIM / newWidth))
        newWidth = MAX_UPLOAD_DIM
      }
      if (newHeight > MAX_UPLOAD_DIM) {
        newWidth = Math.round(newWidth * (MAX_UPLOAD_DIM / newHeight))
        newHeight = MAX_UPLOAD_DIM
      }

      const canvas = document.createElement('canvas')
      canvas.width = newWidth
      canvas.height = newHeight
      canvas.getContext('2d').drawImage(img, 0, 0, newWidth, newHeight)

      canvas.toBlob((blob) => {
        resolve(blob ? new File([blob], file.name, { type: file.type }) : file)
      }, file.type, 0.92)
    }

    img.onerror = () => {
      URL.revokeObjectURL(url)
      resolve(file)
    }

    img.src = url
  })
}

/**
 * Upload an image file to the server and return the URL
 */
const uploadImage = async (file) => {
  file = await resizeImage(file)
  isUploadingImage.value = true
  try {
    const response = await api.uploadMedia({
      files: file,
      inline: true,
      linked_model: 'messages'
    })
    return response.data.data.url
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message || 'Failed to upload image'
    })
    return null
  } finally {
    isUploadingImage.value = false
  }
}

/**
 * Insert an image into the editor at the current cursor position
 */
const insertImage = (url) => {
  if (url && editor.value) {
    editor.value.chain().focus().setImage({ src: url }).run()
  }
}

/**
 * Handle paste events to capture images from clipboard
 */
const handlePaste = (view, event) => {
  const items = event.clipboardData?.items
  if (!items) return false

  for (const item of items) {
    if (item.type.startsWith('image/')) {
      event.preventDefault()
      const file = item.getAsFile()
      if (file) {
        uploadImage(file).then((url) => {
          if (url) insertImage(url)
        })
      }
      return true
    }
  }
  return false
}

/**
 * Handle drop events for drag & drop images
 */
const handleDrop = (view, event) => {
  const files = event.dataTransfer?.files
  if (!files || files.length === 0) return false

  const imageFiles = []
  const otherFiles = []
  for (const file of files) {
    if (file.type.startsWith('image/')) {
      imageFiles.push(file)
    } else {
      otherFiles.push(file)
    }
  }

  if (imageFiles.length === 0 && otherFiles.length === 0) return false
  event.preventDefault()

  // Insert images inline
  for (const file of imageFiles) {
    uploadImage(file).then((url) => {
      if (url) insertImage(url)
    })
  }

  // Emit non-image files for attachment upload
  if (otherFiles.length > 0) {
    emit('filesDropped', otherFiles)
  }

  return true
}

/**
 * Trigger the hidden file input for image selection
 */
const triggerImageUpload = () => {
  imageInput.value?.click()
}

/**
 * Handle image selection from file input
 */
const handleImageSelect = async (event) => {
  const file = event.target.files?.[0]
  if (file && file.type.startsWith('image/')) {
    const url = await uploadImage(file)
    if (url) {
      insertImage(url)
    }
  }
  // Reset the input so the same file can be selected again
  event.target.value = ''
}

// Custom table extensions with inline styles for email compatibility
const CustomTable = Table.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      style: {
        parseHTML: (element) =>
          (element.getAttribute('style') || '') +
          '; border: 1px solid #dee2e6 !important; width: 100%; margin:0; table-layout: fixed; border-collapse: collapse; position:relative; border-radius: 0.25rem;'
      }
    }
  }
})

const CustomTableCell = TableCell.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      style: {
        parseHTML: (element) =>
          (element.getAttribute('style') || '') +
          '; border: 1px solid #dee2e6 !important; box-sizing: border-box !important; min-width: 1em !important; padding: 6px 8px !important; vertical-align: top !important;'
      }
    }
  }
})

const CustomTableHeader = TableHeader.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      style: {
        parseHTML: (element) =>
          (element.getAttribute('style') || '') +
          '; background-color: #f8f9fa !important; color: #212529 !important; font-weight: bold !important; text-align: left !important; border: 1px solid #dee2e6 !important; padding: 6px 8px !important;'
      }
    }
  }
})

// Extend Mention to include 'type' attribute for agent/team distinction
const CustomMention = Mention.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      type: {
        default: null,
        parseHTML: (element) => element.getAttribute('data-type'),
        renderHTML: (attributes) => {
          if (!attributes.type) return {}
          return { 'data-type': attributes.type }
        }
      }
    }
  }
})

// Custom Image extension with drag-handle resizing and size presets
const ResizableImage = Image.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      width: {
        default: null,
        parseHTML: element => element.getAttribute('width') || element.style.width?.replace('px', '') || null,
        renderHTML: attributes => {
          if (!attributes.width) return {}
          return { width: attributes.width, style: `width: ${attributes.width}px` }
        }
      },
      height: {
        default: null,
        parseHTML: element => element.getAttribute('height') || null,
        renderHTML: attributes => {
          if (!attributes.height) return {}
          return { height: attributes.height }
        }
      }
    }
  },
  addNodeView() {
    return ({ node, getPos, editor: nodeEditor }) => {
      // Wrapper
      const wrapper = document.createElement('div')
      wrapper.classList.add('image-resizer')
      wrapper.style.display = 'inline-block'
      wrapper.style.position = 'relative'
      wrapper.style.lineHeight = '0'

      // Image
      const img = document.createElement('img')
      img.src = node.attrs.src
      img.alt = node.attrs.alt || ''
      img.title = node.attrs.title || ''
      img.classList.add('inline-image')
      img.style.maxWidth = '100%'
      img.style.height = 'auto'
      if (node.attrs.width) {
        img.style.width = node.attrs.width + 'px'
      }
      wrapper.appendChild(img)

      // Size toolbar (Small / Best fit / Original)
      const toolbar = document.createElement('div')
      toolbar.classList.add('image-size-toolbar')
      const sizes = [
        { label: 'Small', value: 200 },
        { label: 'Best fit', value: 'fit' },
        { label: 'Original', value: 'original' },
      ]
      let naturalWidth = 0
      img.addEventListener('load', () => { naturalWidth = img.naturalWidth })

      const commitWidth = (newWidth) => {
        const pos = getPos()
        if (typeof pos === 'number') {
          nodeEditor.chain().focus().command(({ tr }) => {
            tr.setNodeMarkup(pos, undefined, { ...node.attrs, width: newWidth || null })
            return true
          }).run()
        }
      }

      sizes.forEach(({ label, value }) => {
        const btn = document.createElement('button')
        btn.textContent = label
        btn.type = 'button'
        btn.addEventListener('mousedown', (e) => {
          e.preventDefault()
          e.stopPropagation()
          if (value === 'original') {
            img.style.width = naturalWidth ? naturalWidth + 'px' : 'auto'
            commitWidth(naturalWidth || null)
          } else if (value === 'fit') {
            img.style.width = ''
            commitWidth(null)
          } else {
            img.style.width = value + 'px'
            commitWidth(value)
          }
        })
        toolbar.appendChild(btn)
      })
      wrapper.appendChild(toolbar)

      // Resize handle (bottom-right corner)
      const handle = document.createElement('div')
      handle.classList.add('image-resize-handle')
      wrapper.appendChild(handle)

      // Only show handle + toolbar when wrapper is selected
      wrapper.addEventListener('click', (e) => {
        e.stopPropagation()
        wrapper.classList.add('selected')
      })

      const onDocClick = (e) => {
        if (!wrapper.contains(e.target)) {
          wrapper.classList.remove('selected')
        }
      }
      document.addEventListener('click', onDocClick)

      // Drag to resize
      let startX = 0
      let startWidth = 0

      const onMouseDown = (e) => {
        e.preventDefault()
        e.stopPropagation()
        startX = e.clientX
        startWidth = img.offsetWidth
        document.addEventListener('mousemove', onMouseMove)
        document.addEventListener('mouseup', onMouseUp)
        wrapper.classList.add('resizing')
      }

      const onMouseMove = (e) => {
        const diff = e.clientX - startX
        const newWidth = Math.max(50, startWidth + diff)
        img.style.width = newWidth + 'px'
      }

      const onMouseUp = () => {
        document.removeEventListener('mousemove', onMouseMove)
        document.removeEventListener('mouseup', onMouseUp)
        wrapper.classList.remove('resizing')
        commitWidth(Math.round(img.offsetWidth))
      }

      handle.addEventListener('mousedown', onMouseDown)

      return {
        dom: wrapper,
        update: (updatedNode) => {
          if (updatedNode.type.name !== 'image') return false
          img.src = updatedNode.attrs.src
          if (updatedNode.attrs.width) {
            img.style.width = updatedNode.attrs.width + 'px'
          } else {
            img.style.width = ''
          }
          return true
        },
        destroy: () => {
          handle.removeEventListener('mousedown', onMouseDown)
          document.removeEventListener('click', onDocClick)
        }
      }
    }
  }
})

const isInternalUpdate = ref(false)

const buildExtensions = () => {
  const extensions = [
    StarterKit.configure(),
    ListExitExtension,
    Underline,
    TextStyle,
    Color,
    Highlight.configure({ multicolor: true }),
    TextAlign.configure({ types: ['heading', 'paragraph'] }),
    ResizableImage.configure({
      HTMLAttributes: { class: 'inline-image', style: 'max-width: 100%; height: auto;' },
      allowBase64: false,
    }),
    Placeholder.configure({ placeholder: () => props.placeholder }),
    Link,
    CustomTable.configure({ resizable: false }),
    TableRow,
    CustomTableCell,
    CustomTableHeader,
    // Always include mention extension - it gracefully handles missing getSuggestions
    CustomMention.configure({
      HTMLAttributes: {
        class: 'mention'
      },
      suggestion: mentionSuggestion
    })
  ]

  return extensions
}

// Extract mentions from editor content
const extractMentions = () => {
  if (!editor.value) return []
  const mentions = []
  const json = editor.value.getJSON()

  const traverse = (node) => {
    if (node.type === 'mention' && node.attrs) {
      mentions.push({
        id: node.attrs.id,
        type: node.attrs.type
      })
    }
    if (node.content) {
      node.content.forEach(traverse)
    }
  }

  if (json.content) {
    json.content.forEach(traverse)
  }

  return mentions
}


const editor = useEditor({
  extensions: buildExtensions(),
  autofocus: props.autoFocus ? 'start' : false,
  content: htmlContent.value,
  editorProps: {
    attributes: { class: 'outline-none' },
    getSuggestions: props.getSuggestions,
    handlePaste,
    handleDrop,
    handleKeyDown: (view, event) => {
      if (event.ctrlKey && event.key.toLowerCase() === 'b') {
        event.stopPropagation()
        return false
      }
      if (event.ctrlKey && event.key === 'Enter') {
        emit('send')
        return true
      }
      // Enter-in-empty-list-item handled by ListExitExtension (ProseMirror keymap).
    }
  },
  onUpdate: ({ editor }) => {
    isInternalUpdate.value = true
    htmlContent.value = editor.getHTML()
    textContent.value = editor.getText()
    isInternalUpdate.value = false

    // Emit mentions if enabled
    if (props.enableMentions) {
      emit('mentionsChanged', extractMentions())
    }
  }
})



watch(
  htmlContent,
  (newContent) => {
    if (isInternalUpdate.value || !editor.value) return
    if (newContent === editor.value.getHTML()) return
    const wasFocused = editor.value.isFocused
    editor.value.commands.setContent(newContent || '', false)
    textContent.value = editor.value.getText()
    if (!wasFocused) editor.value.commands.focus('start')
  },
  { immediate: true, flush: 'sync' }
)

watch(
  () => props.insertContent,
  (val) => {
    if (val && editor.value) {
      // Focus editor, restoring last cursor position so macros insert where the user left off.
      // If editor had no prior selection (fresh), TipTap defaults to start so we fall back to end.
      if (!editor.value.isFocused) {
        const hadSelection = editor.value.state.selection && editor.value.state.selection.anchor > 0
        editor.value.commands.focus(hadSelection ? null : 'end')
      }
      editor.value.commands.insertContent(val)
    }
  }
)

onUnmounted(() => {
  editor.value?.destroy()
})

const openLinkModal = () => {
  if (editor.value?.isActive('link')) {
    linkUrl.value = editor.value.getAttributes('link').href
  } else {
    linkUrl.value = ''
  }
  showLinkDialog.value = true
}

const setLink = () => {
  if (linkUrl.value) {
    editor.value?.chain().focus().extendMarkRange('link').setLink({ href: linkUrl.value }).run()
  }
  showLinkDialog.value = false
}

const unsetLink = () => {
  editor.value?.chain().focus().unsetLink().run()
  showLinkDialog.value = false
}

// Expose focus method for parent components
const focus = () => {
  editor.value?.commands.focus()
}

const runCommand = (command, arg) => {
  if (!editor.value) return
  const c = editor.value.chain().focus()
  switch (command) {
    case 'toggleBold': c.toggleBold().run(); break
    case 'toggleItalic': c.toggleItalic().run(); break
    case 'toggleUnderline': c.toggleUnderline().run(); break
    case 'toggleStrike': c.toggleStrike().run(); break
    case 'toggleBulletList': c.toggleBulletList().run(); break
    case 'toggleOrderedList': c.toggleOrderedList().run(); break
    case 'setColor': c.setColor(arg).run(); break
    case 'unsetColor': c.unsetColor().run(); break
    case 'toggleHighlight': c.toggleHighlight({ color: arg }).run(); break
    case 'unsetHighlight': c.unsetHighlight().run(); break
    case 'setTextAlign': c.setTextAlign(arg).run(); break
    case 'openLink': openLinkModal(); break
    case 'insertImage': triggerImageUpload(); break
  }
}

defineExpose({ focus, extractMentions, editor, runCommand })
</script>

<style lang="scss">
.tiptap p.is-editor-empty:first-child::before {
  content: attr(data-placeholder);
  float: left;
  color: #adb5bd;
  pointer-events: none;
  height: 0;
  font-size: 0.875rem;
}

.editor-wrapper div[aria-expanded='false'] {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.tiptap.ProseMirror {
  p {
    margin: 0;
  }

  flex: 1;
  min-height: 70px;
  overflow-y: auto;
  word-wrap: break-word !important;
  overflow-wrap: break-word !important;
  word-break: break-word;
  white-space: pre-wrap;
  max-width: 100%;
}

.tiptap {
  .tableWrapper {
    margin: 1.5rem 0;
    overflow-x: auto;
  }

  a {
    color: #0066cc;
    cursor: pointer;

    &:hover {
      color: #003d7a;
    }
  }

// Mention styling
  .mention {
    background-color: hsl(var(--primary) / 0.1);
    border-radius: 0.25rem;
    padding: 0.125rem 0.25rem;
    color: hsl(var(--primary));
    font-weight: 500;
  }

  // Inline image styling
  .inline-image {
    max-width: 100%;
    height: auto;
    border-radius: 4px;
    margin: 8px 0;
    cursor: pointer;

    &:hover {
      outline: 2px solid #0066cc;
    }
  }

  // Image selected state
  .ProseMirror-selectednode .inline-image {
    outline: 2px solid #0066cc;
  }

  // Image resizer wrapper
  .image-resizer {
    display: inline-block;
    position: relative;
    margin: 4px 0;

    .image-resize-handle {
      display: none;
      position: absolute;
      bottom: 4px;
      right: 4px;
      width: 12px;
      height: 12px;
      background: #0066cc;
      border: 2px solid white;
      border-radius: 2px;
      cursor: nwse-resize;
      z-index: 10;
    }

    // Size toolbar
    .image-size-toolbar {
      display: none;
      position: absolute;
      bottom: -32px;
      left: 50%;
      transform: translateX(-50%);
      background: hsl(var(--background));
      border: 1px solid hsl(var(--border));
      border-radius: 6px;
      padding: 2px;
      z-index: 20;
      white-space: nowrap;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.12);

      button {
        padding: 2px 8px;
        font-size: 11px;
        color: hsl(var(--muted-foreground));
        background: none;
        border: none;
        border-radius: 4px;
        cursor: pointer;
        line-height: 1.6;

        &:hover {
          background: hsl(var(--accent));
          color: hsl(var(--accent-foreground));
        }
      }
    }

    &.selected .image-resize-handle,
    &.resizing .image-resize-handle {
      display: block;
    }

    &.selected .image-size-toolbar {
      display: flex;
    }

    &.selected .inline-image {
      outline: 2px solid #0066cc;
    }

    &.resizing .inline-image {
      outline: 2px solid #0066cc;
      opacity: 0.8;
    }
  }

  // Email signature styling
  .email-signature {
    border-top: 1px solid #e5e7eb;
    margin-top: 1rem;
    padding-top: 0.75rem;
    color: #6b7280;
    font-size: 0.875rem;
  }
}
</style>

