<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Refresh, Tickets } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import MassageCycleCard from '@/components/massage/MassageCycleCard.vue'
import { massageService } from '@/services/massage'
import type { MassageMe } from '@/types/domain'

const items = ref<MassageMe[]>([])
const loading = ref(false)
const acting = ref('')
async function load() { loading.value = true; try { items.value = await massageService.me() } finally { loading.value = false } }
async function respond(item: MassageMe, action: 'enroll' | 'decline' | 'withdraw') {
  try {
    if (action === 'withdraw') await ElMessageBox.confirm('退出后重新报名会排到队尾，确认退出吗？', '退出按摩排号', { type: 'warning' })
    acting.value = item.cycle.id
    await massageService.respond(item.cycle.id, action)
    ElMessage.success(action === 'enroll' ? '报名成功' : action === 'withdraw' ? '已退出排号' : '已记录本月不参加')
    await load()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(error instanceof Error ? error.message : '操作失败')
  } finally { acting.value = '' }
}
onMounted(load)
</script>

<template>
  <section class="massages-page">
    <header class="page-hero"><div class="hero-icon"><Tickets /></div><div><p>PERSONAL WELLNESS</p><h1>我的按摩</h1><span>报名每月按摩服务，查看自己的排号与当前叫号进度</span></div><el-button :icon="Refresh" :loading="loading" @click="load">刷新</el-button></header>
    <div v-loading="loading" class="cycle-list"><el-empty v-if="!loading && !items.length" description="当前没有面向你的按摩批次" /><MassageCycleCard v-for="item in items" :key="item.cycle.id" :item="item" :loading="acting === item.cycle.id" @respond="respond" /></div>
  </section>
</template>

<style scoped>
.massages-page{max-width:1100px;margin:0 auto;padding:28px}.page-hero{display:flex;align-items:center;gap:15px;margin-bottom:20px;padding:22px 24px;border-radius:20px;color:#fff;background:linear-gradient(125deg,#47378e,#7c61d5 65%,#b17ee5);box-shadow:0 18px 42px rgba(88,65,176,.24)}.hero-icon{display:grid;place-items:center;width:52px;height:52px;border-radius:16px;background:rgba(255,255,255,.16)}.hero-icon svg{width:27px}.page-hero>div:nth-child(2){flex:1}.page-hero p{margin:0 0 2px;font-size:11px;letter-spacing:.16em;opacity:.72}.page-hero h1{margin:0 0 3px;font-size:25px}.page-hero span{font-size:13px;opacity:.82}.cycle-list{display:grid;gap:16px;min-height:180px}@media(max-width:700px){.massages-page{padding:16px}.page-hero{align-items:flex-start;flex-wrap:wrap}.page-hero>.el-button{margin-left:67px}}
</style>
