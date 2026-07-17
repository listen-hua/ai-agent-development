<script setup lang="ts">defineProps<{ label: string; value: string | number; note?: string; tone?: 'blue' | 'green' | 'amber' | 'violet' }>()</script>
<template><article class="metric-card" :class="`tone-${tone || 'blue'}`"><span>{{ label }}</span><strong>{{ value }}</strong><small v-if="note">{{ note }}</small></article></template>
