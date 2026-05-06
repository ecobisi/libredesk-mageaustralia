<template>
  <!-- v1 reserved 14rem of side padding on xl viewports which squeezed the
    actor / target / timestamp columns to ~30% of the screen on a 1080p
    monitor. Tightened so the activity table has breathing room without
    forcing horizontal scroll. -->
  <div class="w-full h-screen p-0 sm:px-8 md:px-12 lg:px-16 xl:px-24">
    <slot></slot>
  </div>
</template>
