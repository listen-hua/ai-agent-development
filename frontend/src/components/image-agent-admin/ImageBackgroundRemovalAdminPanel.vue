<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { useBackgroundRemovalAdmin } from '@/composables/image-agent/useBackgroundRemovalAdmin'

interface PixianForm {
  enabled: boolean
  test_mode: boolean
  api_id: string
  api_secret: string
  timeout_seconds: number
  concurrency: number
  max_pixels: number
}

const admin = useBackgroundRemovalAdmin()
const formRef = ref<FormInstance>()
const form = reactive<PixianForm>({ enabled: false, test_mode: true, api_id: '', api_secret: '', timeout_seconds: 180, concurrency: 2, max_pixels: 25_000_000 })
const rules: FormRules<PixianForm> = {
  timeout_seconds: [{ required: true, type: 'number', min: 180, max: 600, message: '请求超时必须在 180 到 600 秒之间', trigger: 'change' }],
  concurrency: [{ required: true, type: 'number', min: 1, max: 5, message: '全局并发必须在 1 到 5 之间', trigger: 'change' }],
  max_pixels: [{ required: true, type: 'number', min: 100, max: 25_000_000, message: '最大输出像素必须在 100 到 25000000 之间', trigger: 'change' }],
}

watch(admin.config, (value) => {
  if (!value) return
  Object.assign(form, {
    enabled: value.enabled,
    test_mode: value.test_mode,
    timeout_seconds: value.timeout_seconds,
    concurrency: value.concurrency,
    max_pixels: value.max_pixels,
  })
}, { immediate: true })

const accountChecked = computed(() => admin.config.value?.account_checked_at ? new Date(admin.config.value.account_checked_at).toLocaleString() : '尚未检查')
const credentialDirty = computed(() => Boolean(form.api_id.trim() || form.api_secret.trim()))
const testDisabledReason = computed(() => {
  if (credentialDirty.value) return '检测到尚未保存的新凭证，请先保存配置'
  if (!admin.credentialsConfigured.value) return '请先保存 API ID 和 API Secret'
  return ''
})

async function saveConfig() {
  if (!admin.config.value) {
    ElMessage.error('配置尚未加载成功，请先刷新页面数据')
    return
  }
  if (!await formRef.value?.validate().catch(() => false)) return
  const hasID = admin.config.value.has_api_id || Boolean(form.api_id.trim())
  const hasSecret = admin.config.value.has_api_secret || Boolean(form.api_secret.trim())
  if (form.enabled && (!hasID || !hasSecret)) {
    ElMessage.warning('启用抠图服务前必须先配置 API ID 和 API Secret')
    return
  }
  const saved = await admin.save({
    ...form,
    api_id: form.api_id.trim() || undefined,
    api_secret: form.api_secret.trim() || undefined,
  })
  if (saved) {
    form.api_id = ''
    form.api_secret = ''
    formRef.value?.clearValidate(['api_id', 'api_secret'])
  }
}

async function testConnection(refresh: boolean) {
  if (testDisabledReason.value) {
    ElMessage.warning(testDisabledReason.value)
    return
  }
  await admin.test(refresh)
}

onMounted(admin.load)
</script>
<template>
  <section class="admin-feature-panel pixian-panel">
    <header><div><h3>Pixian 智能抠图</h3><p>框选画布图片后生成透明 PNG。图片会发送至 Pixian 外部服务，请在启用前完成公司数据合规确认。</p></div><el-button :icon="Refresh" :loading="admin.loading.value" @click="admin.load">刷新</el-button></header>
    <el-alert type="warning" :closable="false" show-icon title="默认关闭并使用测试模式；测试模式会产生带水印结果但不扣生产 Credits。" />
    <el-alert v-if="admin.loadError.value" type="error" :closable="false" show-icon :title="admin.loadError.value" />
    <el-form ref="formRef" :model="form" :rules="rules" :disabled="!admin.config.value || admin.loading.value" label-position="top" class="pixian-form"><div class="form-grid">
      <el-form-item label="全局启用"><el-switch v-model="form.enabled" /></el-form-item><el-form-item label="测试模式"><el-switch v-model="form.test_mode" /></el-form-item>
      <el-form-item label="API ID" prop="api_id"><el-input v-model="form.api_id" autocomplete="off" :placeholder="admin.config.value?.has_api_id ? `已配置 ${admin.config.value.api_id_hint}，留空保持` : '请输入 Pixian API ID'" /></el-form-item>
      <el-form-item label="API Secret" prop="api_secret"><el-input v-model="form.api_secret" autocomplete="new-password" type="password" show-password :placeholder="admin.config.value?.has_api_secret ? `已配置 ${admin.config.value.api_secret_hint}，留空保持` : '请输入 Pixian API Secret'" /></el-form-item>
      <el-form-item label="请求超时（秒）" prop="timeout_seconds"><el-input-number v-model="form.timeout_seconds" :min="180" :max="600" /></el-form-item><el-form-item label="全局并发" prop="concurrency"><el-input-number v-model="form.concurrency" :min="1" :max="5" /></el-form-item>
      <el-form-item label="最大输出像素" prop="max_pixels"><el-input-number v-model="form.max_pixels" :min="100" :max="25000000" :step="100000" /></el-form-item>
    </div><div class="form-actions"><el-button type="primary" :loading="admin.saving.value" @click="saveConfig">保存配置</el-button><el-tooltip :content="testDisabledReason" :disabled="!testDisabledReason"><span><el-button :disabled="Boolean(testDisabledReason)" :loading="admin.testingAction.value === 'test'" @click="testConnection(false)">测试连接</el-button></span></el-tooltip><el-tooltip :content="testDisabledReason" :disabled="!testDisabledReason"><span><el-button :disabled="Boolean(testDisabledReason)" :loading="admin.testingAction.value === 'refresh'" @click="testConnection(true)">刷新余额</el-button></span></el-tooltip></div></el-form>
    <div class="stat-grid"><article><span>账户状态</span><strong>{{ admin.config.value?.account_state || '未知' }}</strong><small>{{ accountChecked }}</small></article><article><span>剩余 Credits</span><strong>{{ admin.config.value?.account_credits ?? 0 }}</strong><small>以 Pixian 账户接口为准</small></article><article><span>今日调用</span><strong>{{ admin.statistics.value?.today_calls ?? 0 }}</strong><small>批次任务</small></article><article><span>30 天处理图片</span><strong>{{ admin.statistics.value?.thirty_day_images ?? 0 }}</strong><small>成功与失败合计</small></article><article><span>实际扣费</span><strong>{{ admin.statistics.value?.credits_charged ?? 0 }}</strong><small>Credits</small></article><article><span>计算费用</span><strong>{{ admin.statistics.value?.credits_calculated ?? 0 }}</strong><small>含测试任务</small></article></div>
    <h4>最近任务</h4><el-table :data="admin.jobs.value" v-loading="admin.loading.value" size="small"><el-table-column prop="created_at" label="创建时间" min-width="170"><template #default="scope">{{ new Date(scope.row.created_at).toLocaleString() }}</template></el-table-column><el-table-column prop="status" label="状态" width="110" /><el-table-column label="图片" width="100"><template #default="scope">{{ scope.row.completed_count + scope.row.failed_count }}</template></el-table-column><el-table-column prop="credits_charged" label="实际 Credits" width="130" /><el-table-column prop="error" label="错误" min-width="220" show-overflow-tooltip /></el-table>
  </section>
</template>
<style scoped>.pixian-panel{display:grid;gap:18px}.pixian-form{padding:16px;border-radius:12px;background:#faf9fc}.form-actions{display:flex;gap:10px}.stat-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}.stat-grid article{padding:14px;border:1px solid #ece7f2;border-radius:12px;background:#fff;display:flex;flex-direction:column;gap:4px}.stat-grid span,.stat-grid small{color:#8d8594;font-size:11px}.stat-grid strong{color:#4e3e62;font-size:21px}h4{margin:0;color:#4a4054}@media(max-width:800px){.stat-grid{grid-template-columns:1fr 1fr}}</style>
