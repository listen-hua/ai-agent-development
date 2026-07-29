<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
</script>

<template>
  <main v-if="auth.isIAMInitializing" class="iam-bootstrap">
    <span class="iam-bootstrap__spinner" />
    <strong>正在连接公司 IAM</strong>
    <small>正在确认账号与应用权限，请稍候</small>
  </main>
  <RouterView v-else />
</template>
