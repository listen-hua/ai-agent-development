<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import type { DirectoryOptions, MassageCycle } from '@/types/domain'
import type { MassageCycleInput } from '@/services/massage'
import { validateMassageCycleInput } from '@/utils/massageCycle'

const props = defineProps<{ modelValue: boolean; cycle?: MassageCycle; directory: DirectoryOptions; saving: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; save: [value: MassageCycleInput] }>()
const fresh = (): MassageCycleInput => ({
  service_month: new Date().toISOString().slice(0, 7),
  title: '员工按摩服务',
  signup_notice_at: '',
  signup_deadline: '',
  audience: { scope: 'all', department_ids: [], user_ids: [], excluded_user_ids: [] },
  sessions: [
    { sequence: 1, starts_at: '', quota: 50, concurrent_slots: 1 },
    { sequence: 2, starts_at: '', quota: 50, concurrent_slots: 1 },
  ],
})
const form = reactive<MassageCycleInput>(fresh())
const validationError = ref('')

watch(() => props.modelValue, (open) => {
  if (!open) return
  validationError.value = ''
  Object.assign(form, fresh())
  if (!props.cycle) return
  form.service_month = props.cycle.service_month
  form.title = props.cycle.title
  form.signup_notice_at = props.cycle.signup_notice_at
  form.signup_deadline = props.cycle.signup_deadline
  form.audience = {
    scope: props.cycle.audience.scope,
    department_ids: [...(props.cycle.audience.department_ids || [])],
    user_ids: [...(props.cycle.audience.user_ids || [])],
    excluded_user_ids: [...(props.cycle.audience.excluded_user_ids || [])],
  }
  form.sessions = props.cycle.sessions.map(session => ({
    id: session.id,
    sequence: session.sequence,
    starts_at: session.starts_at,
    quota: session.quota,
    concurrent_slots: session.concurrent_slots,
  }))
})

function submit() {
  const result = validateMassageCycleInput({
    ...form,
    audience: {
      ...form.audience,
      department_ids: [...form.audience.department_ids],
      user_ids: [...form.audience.user_ids],
      excluded_user_ids: [...form.audience.excluded_user_ids],
    },
    sessions: form.sessions.map(session => ({ ...session })),
  })
  if (!result.payload) {
    validationError.value = result.error || '请检查批次信息'
    ElMessage.error(validationError.value)
    return
  }
  validationError.value = ''
  emit('save', result.payload)
}
</script>

<template>
  <el-dialog :model-value="modelValue" :title="cycle ? '编辑按摩批次' : '新建按摩批次'" width="760px" @update:model-value="emit('update:modelValue', $event)">
    <el-alert v-if="validationError" class="validation-alert" :title="validationError" type="error" :closable="false" show-icon />
    <el-form label-position="top" @change="validationError = ''">
      <div class="form-grid">
        <el-form-item label="服务月份"><el-date-picker v-model="form.service_month" type="month" value-format="YYYY-MM" /></el-form-item>
        <el-form-item label="批次名称"><el-input v-model="form.title" maxlength="100" /></el-form-item>
        <el-form-item label="报名通知时间"><el-date-picker v-model="form.signup_notice_at" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" /></el-form-item>
        <el-form-item label="报名截止时间"><el-date-picker v-model="form.signup_deadline" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" /></el-form-item>
      </div>
      <el-form-item label="参与范围">
        <el-radio-group v-model="form.audience.scope">
          <el-radio-button value="all">全体在职员工</el-radio-button>
          <el-radio-button value="restricted">指定部门或人员</el-radio-button>
        </el-radio-group>
      </el-form-item>
      <template v-if="form.audience.scope === 'restricted'">
        <el-form-item label="可报名部门"><el-select v-model="form.audience.department_ids" multiple filterable collapse-tags><el-option v-for="department in directory.departments" :key="department.id" :label="department.path" :value="department.id" /></el-select></el-form-item>
        <el-form-item label="额外指定人员"><el-select v-model="form.audience.user_ids" multiple filterable><el-option v-for="user in directory.users" :key="user.id" :label="`${user.name} · ${user.job_title || '未设置职位'}`" :value="user.id" /></el-select></el-form-item>
      </template>
      <el-form-item label="排除人员"><el-select v-model="form.audience.excluded_user_ids" multiple filterable><el-option v-for="user in directory.users" :key="user.id" :label="user.name" :value="user.id" /></el-select></el-form-item>
      <section v-for="session in form.sessions" :key="session.sequence" class="session-form">
        <h4>第 {{ session.sequence }} 场</h4>
        <div class="form-grid three">
          <el-form-item label="计划时间"><el-date-picker v-model="session.starts_at" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" /></el-form-item>
          <el-form-item label="本场人数"><el-input-number v-model="session.quota" :min="1" :max="10000" /></el-form-item>
          <el-form-item label="并行服务人数"><el-input-number v-model="session.concurrent_slots" :min="1" :max="20" /></el-form-item>
        </div>
      </section>
    </el-form>
    <template #footer><el-button @click="emit('update:modelValue', false)">取消</el-button><el-button type="primary" :loading="saving" @click="submit">保存批次</el-button></template>
  </el-dialog>
</template>

<style scoped>
.validation-alert{margin-bottom:16px}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:0 16px}.form-grid.three{grid-template-columns:2fr 1fr 1fr}.el-select,.el-date-editor{width:100%}.session-form{margin-top:12px;padding:14px 16px 0;border:1px solid #ebe7f2;border-radius:14px;background:#faf8fd}.session-form h4{margin:0 0 10px;color:#4f4078}@media(max-width:700px){.form-grid,.form-grid.three{grid-template-columns:1fr}}
</style>
