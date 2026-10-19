<script setup lang="ts">
defineProps<{ name: 'library' | 'avatar' | 'tag' | 'settings' }>();
</script>

<template>
  <svg
    class="nav-symbol"
    viewBox="0 0 24 24"
    width="20"
    height="20"
    fill="none"
    stroke="currentColor"
    stroke-width="1.7"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    focusable="false"
  >
    <template v-if="name === 'library'">
      <rect x="3" y="3" width="7" height="7" rx="1.5" />
      <rect x="14" y="3" width="7" height="7" rx="1.5" />
      <rect x="3" y="14" width="7" height="7" rx="1.5" />
      <rect x="14" y="14" width="7" height="7" rx="1.5" />
    </template>
    <template v-else-if="name === 'avatar'">
      <circle cx="12" cy="8" r="4" />
      <path d="M4 21v-2a8 8 0 0 1 16 0v2M8 20h8" />
    </template>
    <template v-else-if="name === 'tag'">
      <path d="M3 4v7a2 2 0 0 0 .6 1.4l8 8a2 2 0 0 0 2.8 0l6-6a2 2 0 0 0 0-2.8l-8-8A2 2 0 0 0 11 3H4a1 1 0 0 0-1 1Z" />
      <circle cx="7.5" cy="7.5" r="1.2" />
    </template>
    <template v-else>
      <path d="m10 3-.5 2.3-1.8 1L5.5 6l-2 3.5L5 11.3v1.4l-1.5 1.8 2 3.5 2.2-.3 1.8 1L10 21h4l.5-2.3 1.8-1 2.2.3 2-3.5-1.5-1.8v-1.4l1.5-1.8-2-3.5-2.2.3-1.8-1L14 3Z" />
      <circle cx="12" cy="12" r="3" />
    </template>
  </svg>
</template>
