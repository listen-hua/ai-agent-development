<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Connection, Lock, Sunny, UserFilled } from '@element-plus/icons-vue'
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

function retryIAM() {
  window.location.reload()
}
</script>

<template>
  <main class="login-page">
    <section class="login-story">
      <div class="story-inner"><div class="story-badge"><span />SHIMMER · 企业可信 AI</div><h1>让每一个 AI Agent，<br />照亮具体工作。</h1><p>从制度问答、行政执行到创意生图，在飞书内连接公司知识、工具与多个专业 AI Agent。</p><div class="trust-list"><div><Lock /><span><strong>统一身份与权限</strong>每个 Agent 都遵守公司数据边界</span></div><div><Connection /><span><strong>一个入口，多种能力</strong>按业务场景持续接入新的 AI Agent</span></div></div></div>
    </section>
    <section class="login-panel">
      <div class="login-card">
        <div class="login-logo"><Sunny /></div>
        <h2>欢迎使用微光</h2>
        <p>Shimmer · 公司内部 AI Agent 平台</p>
        <el-alert v-if="auth.authError" :title="auth.authError" type="error" show-icon :closable="false" />
        <template v-if="auth.iamConfigured">
          <el-button class="feishu-login" type="primary" size="large" :loading="auth.isIAMInitializing" @click="retryIAM">重新连接公司 IAM</el-button>
          <small>普通浏览器通过公司 IAM 自动登录，无需输入微光账号密码</small>
        </template>
        <template v-else>
          <el-button class="feishu-login" type="primary" size="large" :loading="loading" @click="login('feishu')">使用飞书账号登录</el-button>
        </template>
        <template v-if="auth.devAuthEnabled && !auth.iamConfigured">
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
