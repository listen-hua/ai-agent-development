<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { Delete, Upload } from '@element-plus/icons-vue'
import type { UploadFile } from 'element-plus'
import type { WorkdayOverride } from '@/types/domain'

const props = defineProps<{ items: WorkdayOverride[]; year: number; loading: boolean }>()
const emit = defineEmits<{ yearChange: [year: number]; save: [value: { date: string; is_workday: boolean; note: string }]; remove: [date: string]; import: [file: File] }>()
const calendarDate = ref(new Date(props.year, 0, 1))
const dialogOpen = ref(false)
const form = reactive({ date: '', is_workday: true, note: '' })
const overrides = computed(() => new Map(props.items.map((item) => [item.date, item])))
watch(() => props.year, (year) => { calendarDate.value = new Date(year, 0, 1) })
function selectDate(date: string) { const value = overrides.value.get(date); form.date = date; form.is_workday = value?.is_workday ?? !isWeekend(date); form.note = value?.note || ''; dialogOpen.value = true }
function isWeekend(date: string) { const day = new Date(`${date}T00:00:00+08:00`).getDay(); return day === 0 || day === 6 }
function dayLabel(date: string) { const value = overrides.value.get(date); if (value) return value.is_workday ? '调休上班' : '公司休息'; return isWeekend(date) ? '周末' : '默认工作日' }
function dayTone(date: string) { const value = overrides.value.get(date); if (value) return value.is_workday ? 'work' : 'rest'; return isWeekend(date) ? 'weekend' : '' }
function upload(file: UploadFile) { if (file.raw) emit('import', file.raw); return false }
function changeYear(value: unknown) { if (value instanceof Date) emit('yearChange', value.getFullYear()) }
</script>

<template>
  <div v-loading="loading" class="work-calendar-layout">
    <section class="work-calendar-card">
      <header><div><strong>公司工作日历</strong><small>未覆盖日期默认周一至周五上班</small></div><el-date-picker :model-value="new Date(year, 0, 1)" type="year" format="YYYY 年" @update:model-value="changeYear" /></header>
      <el-calendar v-model="calendarDate"><template #date-cell="{ data }"><button class="calendar-day" :class="dayTone(data.day)" @click.stop="selectDate(data.day)"><span>{{ data.day.slice(-2) }}</span><small>{{ dayLabel(data.day) }}</small></button></template></el-calendar>
    </section>
    <aside class="work-calendar-side"><div><strong>本年特殊日期</strong><small>{{ items.length }} 条覆盖</small></div><div v-if="!items.length" class="calendar-empty">尚未配置节假日和调休日期。</div><article v-for="item in items" :key="item.date"><div><strong>{{ item.date }}</strong><small>{{ item.note || (item.is_workday ? '调休上班' : '公司休息') }}</small></div><el-tag size="small" :type="item.is_workday ? 'success' : 'info'">{{ item.is_workday ? '上班' : '休息' }}</el-tag><el-button link type="danger" :icon="Delete" @click="emit('remove', item.date)" /></article><el-upload accept=".csv,text/csv" :auto-upload="false" :show-file-list="false" :on-change="upload"><el-button class="calendar-import" :icon="Upload">导入 CSV</el-button></el-upload><p>列格式：<code>date,is_workday,note</code></p></aside>
    <el-dialog v-model="dialogOpen" title="设置特殊工作日" width="420px"><el-form label-position="top"><el-form-item label="日期"><el-input v-model="form.date" disabled /></el-form-item><el-form-item label="当天安排"><el-radio-group v-model="form.is_workday"><el-radio-button :value="true">上班</el-radio-button><el-radio-button :value="false">休息</el-radio-button></el-radio-group></el-form-item><el-form-item label="说明"><el-input v-model="form.note" maxlength="100" placeholder="例如：国庆节、春节调休" /></el-form-item></el-form><template #footer><el-button @click="dialogOpen = false">取消</el-button><el-button type="primary" @click="emit('save', { ...form }); dialogOpen = false">保存</el-button></template></el-dialog>
  </div>
</template>

<style scoped>
.work-calendar-layout{display:grid;grid-template-columns:minmax(0,1fr) 285px;gap:16px;align-items:start}.work-calendar-card,.work-calendar-side{border:1px solid #e4e8ef;border-radius:14px;background:#fff;box-shadow:0 6px 24px rgba(32,46,74,.04)}.work-calendar-card>header{padding:18px 20px;border-bottom:1px solid #edf0f4;display:flex;justify-content:space-between;align-items:center}.work-calendar-card>header>div,.work-calendar-side>div:first-child{display:flex;flex-direction:column}.work-calendar-card strong,.work-calendar-side strong{color:#344158;font-size:12px}.work-calendar-card small,.work-calendar-side small{color:#929baa;font-size:9px}.work-calendar-card :deep(.el-calendar){--el-calendar-cell-width:auto}.work-calendar-card :deep(.el-calendar__header){display:none}.work-calendar-card :deep(.el-calendar__body){padding:13px}.work-calendar-card :deep(.el-calendar-table td){border-color:#edf0f4}.work-calendar-card :deep(.el-calendar-day){height:72px;padding:0}.calendar-day{width:100%;height:100%;border:0;padding:7px;background:transparent;text-align:left;display:flex;flex-direction:column;gap:5px;cursor:pointer}.calendar-day span{color:#4d586e;font-size:11px}.calendar-day small{width:max-content;border-radius:10px;padding:2px 5px;background:#f2f4f8;color:#929baa;font-size:8px}.calendar-day.work small{background:#e7f7ef;color:#23815b}.calendar-day.rest small{background:#fff0ee;color:#bd5b52}.calendar-day.weekend small{background:#f5f2fa;color:#827491}.work-calendar-side{padding:18px}.work-calendar-side>div:first-child{padding-bottom:13px}.work-calendar-side article{min-height:48px;border-top:1px solid #eef0f4;display:grid;grid-template-columns:1fr auto auto;gap:7px;align-items:center}.work-calendar-side article>div{display:flex;flex-direction:column}.calendar-empty{border-radius:9px;padding:18px 10px;background:#f7f8fb;color:#929baa;font-size:10px;text-align:center}.calendar-import{width:100%;margin-top:16px}.work-calendar-side>p{color:#9aa2b0;font-size:9px}.work-calendar-side code{color:#6073bb}@media(max-width:900px){.work-calendar-layout{grid-template-columns:1fr}.work-calendar-side{order:-1}}
</style>
