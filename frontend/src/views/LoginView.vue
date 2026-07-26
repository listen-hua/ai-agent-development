<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Connection, Lock, Operation, UserFilled } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const loading = ref(false)

async function login(kind: 'feishu' | 'admin' | 'employee') {
  loading.value = true
  try {
    if (kind === 'feishu') await auth.feishuLogin()
    else await auth.devLogin(kind === 'employee')
    await router.replace(String(route.query.redirect || '/chat'))
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="login-page">
    <section class="login-story">
      <div class="story-inner"><div class="story-badge"><span />内部可信 AI</div><h1>让制度被看见，<br />让答案有依据。</h1><p>连接公司制度知识与飞书，让每位同事都能快速获得准确、可追溯的行政答复。</p><div class="trust-list"><div><Lock /><span><strong>权限先于检索</strong>看不到的制度，不会出现在答案里</span></div><div><Connection /><span><strong>每个结论有引用</strong>直达制度版本、章节与原文</span></div></div></div>
    </section>
    <section class="login-panel">
      <div class="login-card">
        <div class="login-logo"><Operation /></div>
        <h2>欢迎使用知行</h2>
        <p>公司内部行政 AI 助手</p>
        <el-alert v-if="auth.authError" :title="auth.authError" type="error" show-icon :closable="false" />
        <el-button class="feishu-login" type="primary" size="large" :loading="loading" @click="login('feishu')">使用飞书账号登录</el-button>
        <template v-if="auth.devAuthEnabled">
          <div class="login-divider"><span>本地开发演示</span></div>
          <div class="demo-actions">
            <el-button :icon="UserFilled" @click="login('admin')">管理员</el-button>
            <el-button @click="login('employee')">普通员工</el-button>
          </div>
        </template>
        <small>登录即表示你同意遵守公司信息安全与 AI 使用规范</small>
      </div>
    </section>
  </main>
</template>
