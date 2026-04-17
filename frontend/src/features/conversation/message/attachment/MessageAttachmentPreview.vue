<template>
  <div class="flex flex-row flex-wrap gap-2 break-all">
    <div
      v-for="attachment in attachments"
      :key="attachment.uuid"
      class="flex items-center cursor-pointer"
    >
      <div>
        <ImageAttachmentPreview v-if="isImage(attachment)" :attachment="attachment" @preview="openLightbox" />
        <div v-else-if="isAudio(attachment)" class="flex items-center gap-2 rounded-lg border bg-gray-50 dark:bg-gray-800 px-3 py-2">
          <audio controls preload="auto" class="h-8 max-w-[260px]">
            <source :src="attachment.url" />
          </audio>
          <a :href="attachment.url" download @click.stop class="p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-600 shrink-0" title="Download">
            <Download class="w-4 h-4 text-gray-500" />
          </a>
        </div>
        <FileAttachmentPreview v-else :attachment="attachment" />
      </div>
    </div>
  </div>

  <!-- Shared lightbox with prev/next + zoom/pan -->
  <Teleport to="body">
    <div
      v-if="lightboxOpen"
      class="fixed inset-0 z-[9999] flex items-center justify-center bg-black/80"
      @click.self="zoomScale === 1 ? (lightboxOpen = false) : resetZoom()"
      @keydown.escape="lightboxOpen = false"
      @keydown.left="prevImage"
      @keydown.right="nextImage"
      @wheel.prevent="handleZoomWheel"
      tabindex="0"
      ref="lightboxEl"
    >
      <!-- Top bar -->
      <div class="absolute top-4 right-4 flex items-center gap-3 z-10">
        <button class="text-white/70 hover:text-white flex items-center gap-1 text-sm" @click.stop="zoomIn" title="Zoom in">
          <ZoomIn :size="20" />
        </button>
        <button class="text-white/70 hover:text-white text-xs font-mono min-w-[3rem] text-center" @click.stop="resetZoom" title="Reset zoom">
          {{ Math.round(zoomScale * 100) }}%
        </button>
        <button class="text-white/70 hover:text-white flex items-center gap-1 text-sm" @click.stop="zoomOut" title="Zoom out">
          <ZoomOut :size="20" />
        </button>
        <a :href="currentImage?.url" download class="text-white/70 hover:text-white" title="Download" @click.stop>
          <Download :size="20" />
        </a>
        <button class="text-white hover:text-gray-300" @click="lightboxOpen = false">
          <X :size="24" />
        </button>
      </div>
      <!-- Counter -->
      <div v-if="imageAttachments.length > 1" class="absolute top-4 left-4 text-white/70 text-sm z-10">
        {{ lightboxIndex + 1 }} / {{ imageAttachments.length }}
      </div>
      <!-- Prev -->
      <button
        v-if="imageAttachments.length > 1"
        class="absolute left-4 top-1/2 -translate-y-1/2 text-white hover:text-gray-300 z-10 p-2"
        @click.stop="prevImage"
      >
        <ChevronLeft :size="32" />
      </button>
      <!-- Next -->
      <button
        v-if="imageAttachments.length > 1"
        class="absolute right-4 top-1/2 -translate-y-1/2 text-white hover:text-gray-300 z-10 p-2"
        @click.stop="nextImage"
      >
        <ChevronRight :size="32" />
      </button>
      <!-- Loading spinner -->
      <div v-if="imageLoading" class="absolute inset-0 flex items-center justify-center pointer-events-none">
        <div class="w-8 h-8 border-2 border-white/30 border-t-white rounded-full animate-spin"></div>
      </div>
      <!-- Zoomable image -->
      <div
        class="overflow-hidden"
        style="max-width: 90vw; max-height: 90vh;"
        @mousedown.prevent="startPan"
        @touchstart.prevent="handleTouchStart"
        @touchmove.prevent="handleTouchMove"
        @touchend="handleTouchEnd"
      >
        <img
          :key="currentImage?.uuid"
          :src="currentImage?.url"
          class="max-w-[90vw] max-h-[90vh] object-contain rounded shadow-2xl select-none transition-opacity duration-150"
          :class="imageLoading ? 'opacity-0' : 'opacity-100'"
          :style="{ transform: `scale(${zoomScale}) translate(${panX / zoomScale}px, ${panY / zoomScale}px)`, cursor: zoomScale > 1 ? 'grab' : 'zoom-in', transition: isPanning ? 'none' : 'transform 0.15s ease' }"
          draggable="false"
          :alt="currentImage?.name"
          @load="imageLoading = false"
          @click.stop="zoomScale === 1 ? zoomIn() : null"
          @dblclick.stop="resetZoom"
        />
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, computed, nextTick, watch } from 'vue'
import ImageAttachmentPreview from '@/features/conversation/message/attachment/ImageAttachmentPreview.vue'
import FileAttachmentPreview from '@/features/conversation/message/attachment/FileAttachmentPreview.vue'
import { Download, X, ChevronLeft, ChevronRight, ZoomIn, ZoomOut } from 'lucide-vue-next'

const props = defineProps({
  attachments: {
    type: Array,
    required: true
  }
})

const isImage = (attachment) => attachment.content_type.includes('image')
const isAudio = (attachment) => attachment.content_type.startsWith('audio/')

const imageAttachments = computed(() => (props.attachments || []).filter(isImage))

const lightboxOpen = ref(false)
const lightboxIndex = ref(0)
const lightboxEl = ref(null)
const imageLoading = ref(false)

const currentImage = computed(() => imageAttachments.value[lightboxIndex.value])

// Zoom & pan state
const zoomScale = ref(1)
const panX = ref(0)
const panY = ref(0)
const isPanning = ref(false)
let panStart = { x: 0, y: 0, panX: 0, panY: 0 }
let lastTouchDist = 0

function resetZoom() {
  zoomScale.value = 1
  panX.value = 0
  panY.value = 0
}

function zoomIn() {
  zoomScale.value = Math.min(zoomScale.value * 1.4, 8)
}

function zoomOut() {
  zoomScale.value = Math.max(zoomScale.value / 1.4, 1)
  if (zoomScale.value === 1) { panX.value = 0; panY.value = 0 }
}

function handleZoomWheel(e) {
  if (e.deltaY < 0) {
    zoomScale.value = Math.min(zoomScale.value * 1.15, 8)
  } else {
    zoomScale.value = Math.max(zoomScale.value / 1.15, 1)
    if (zoomScale.value === 1) { panX.value = 0; panY.value = 0 }
  }
}

function startPan(e) {
  if (zoomScale.value <= 1) return
  isPanning.value = true
  panStart = { x: e.clientX, y: e.clientY, panX: panX.value, panY: panY.value }
  const onMove = (ev) => {
    panX.value = panStart.panX + (ev.clientX - panStart.x)
    panY.value = panStart.panY + (ev.clientY - panStart.y)
  }
  const onUp = () => {
    isPanning.value = false
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
  }
  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
}

function handleTouchStart(e) {
  if (e.touches.length === 2) {
    lastTouchDist = Math.hypot(
      e.touches[0].clientX - e.touches[1].clientX,
      e.touches[0].clientY - e.touches[1].clientY
    )
  } else if (e.touches.length === 1 && zoomScale.value > 1) {
    isPanning.value = true
    panStart = { x: e.touches[0].clientX, y: e.touches[0].clientY, panX: panX.value, panY: panY.value }
  }
}

function handleTouchMove(e) {
  if (e.touches.length === 2) {
    const dist = Math.hypot(
      e.touches[0].clientX - e.touches[1].clientX,
      e.touches[0].clientY - e.touches[1].clientY
    )
    if (lastTouchDist > 0) {
      const delta = dist / lastTouchDist
      zoomScale.value = Math.max(1, Math.min(8, zoomScale.value * delta))
      if (zoomScale.value === 1) { panX.value = 0; panY.value = 0 }
    }
    lastTouchDist = dist
  } else if (e.touches.length === 1 && isPanning.value) {
    panX.value = panStart.panX + (e.touches[0].clientX - panStart.x)
    panY.value = panStart.panY + (e.touches[0].clientY - panStart.y)
  }
}

function handleTouchEnd() {
  isPanning.value = false
  lastTouchDist = 0
}

// Preload adjacent images when index changes
watch(lightboxIndex, () => {
  const imgs = imageAttachments.value
  if (imgs.length <= 1) return
  const prev = (lightboxIndex.value - 1 + imgs.length) % imgs.length
  const next = (lightboxIndex.value + 1) % imgs.length
  new Image().src = imgs[prev].url
  new Image().src = imgs[next].url
})

function openLightbox(attachment) {
  const idx = imageAttachments.value.findIndex(a => a.uuid === attachment.uuid)
  lightboxIndex.value = idx >= 0 ? idx : 0
  imageLoading.value = false
  resetZoom()
  lightboxOpen.value = true
  nextTick(() => {
    lightboxEl.value?.focus()
    // Preload all images when lightbox opens
    imageAttachments.value.forEach(a => { new Image().src = a.url })
  })
}

function prevImage() {
  if (imageAttachments.value.length <= 1) return
  imageLoading.value = true
  resetZoom()
  lightboxIndex.value = (lightboxIndex.value - 1 + imageAttachments.value.length) % imageAttachments.value.length
}

function nextImage() {
  if (imageAttachments.value.length <= 1) return
  imageLoading.value = true
  resetZoom()
  lightboxIndex.value = (lightboxIndex.value + 1) % imageAttachments.value.length
}
</script>
