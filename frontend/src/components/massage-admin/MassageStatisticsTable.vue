<script setup lang="ts">
import type { MassageStatistic } from '@/types/domain'
defineProps<{ value?: MassageStatistic; loading?: boolean }>()
</script>

<template>
  <section v-loading="loading" class="statistics">
    <div class="summary"><article><strong>{{ value?.eligible || 0 }}</strong><span>可报名</span></article><article><strong>{{ value?.enrolled || 0 }}</strong><span>已排号</span></article><article><strong>{{ value?.completed || 0 }}</strong><span>已完成</span></article><article><strong>{{ value?.timed_out || 0 }}</strong><span>超时</span></article><article><strong>{{ value?.no_show || 0 }}</strong><span>未到场</span></article><article><strong>{{ value?.unserved || 0 }}</strong><span>未安排</span></article></div>
    <el-table :data="value?.enrollments || []" height="420"><el-table-column prop="queue_number" label="排号" width="80" /><el-table-column prop="user_name" label="姓名" min-width="120" /><el-table-column prop="status" label="报名状态" width="120" /><el-table-column label="最近叫号状态" min-width="140"><template #default="scope">{{ value?.calls.filter(call => call.enrollment_id === scope.row.id).at(-1)?.status || '尚未叫号' }}</template></el-table-column><el-table-column label="报名时间" min-width="170"><template #default="scope">{{ scope.row.enrolled_at ? new Date(scope.row.enrolled_at).toLocaleString('zh-CN', { hour12: false }) : '-' }}</template></el-table-column></el-table>
  </section>
</template>

<style scoped>
.statistics{min-height:260px}.summary{display:grid;grid-template-columns:repeat(6,1fr);gap:10px;margin-bottom:14px}.summary article{display:flex;flex-direction:column;padding:13px;border:1px solid #ece8f2;border-radius:13px;background:#fff}.summary strong{font-size:23px;color:#59449d}.summary span{font-size:12px;color:#91889d}@media(max-width:850px){.summary{grid-template-columns:repeat(3,1fr)}}
</style>
