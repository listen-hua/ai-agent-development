<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import PageHeader from '@/components/common/PageHeader.vue'
import WorkCalendarEditor from '@/components/reminders/WorkCalendarEditor.vue'
import { workCalendarService } from '@/services/admin'
import type { WorkdayOverride } from '@/types/domain'

const year = ref(new Date().getFullYear())
const items = ref<WorkdayOverride[]>([])
const loading = ref(false)
async function load() { loading.value = true; try { items.value = await workCalendarService.list(year.value) } finally { loading.value = false } }
async function changeYear(value: number) { year.value = value; await load() }
async function save(value: { date: string; is_workday: boolean; note: string }) { await workCalendarService.save(value.date, value); ElMessage.success('公司工作日历已更新'); await load() }
async function remove(date: string) { await ElMessageBox.confirm('删除后将恢复为周一至周五的默认规则。', '删除特殊日期'); await workCalendarService.remove(date); await load() }
async function importCSV(file: File) { const result = await workCalendarService.importCSV(file); ElMessage.success(`已导入 ${result.imported} 条特殊日期`); await load() }
onMounted(load)
</script>

<template><section class="admin-page"><PageHeader eyebrow="COMPANY CALENDAR" title="公司工作日历" description="工作日提醒以这里的调休和节假日配置为准；未覆盖日期默认按周一至周五计算。" /><WorkCalendarEditor :items="items" :year="year" :loading="loading" @year-change="changeYear" @save="save" @remove="remove" @import="importCSV" /></section></template>
