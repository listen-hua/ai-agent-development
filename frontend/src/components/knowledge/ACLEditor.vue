<script setup lang="ts">
import { ref, watch } from 'vue'
import { Delete, Plus } from '@element-plus/icons-vue'
import type { ACL, ACLRule, DirectoryOptions } from '@/types/domain'
import { cloneACL, normalizeACL } from '@/utils/acl'

const props = defineProps<{ modelValue: ACL; options: DirectoryOptions }>()
const emit = defineEmits<{ 'update:modelValue': [value: ACL] }>()
const draft = ref<ACL>(cloneACL(props.modelValue))

watch(() => props.modelValue, (value) => { draft.value = cloneACL(value) }, { deep: true })

function changeScope(scope: ACL['scope']) {
  draft.value = scope === 'all' ? { scope: 'all' } : { scope: 'restricted', rules: [{ department_ids: [], job_titles: [], user_ids: [] }] }
  commit()
}
function addRule() {
  ensureRules().push({ department_ids: [], job_titles: [], user_ids: [] })
  commit()
}
function removeRule(index: number) {
  ensureRules().splice(index, 1)
  if (!ensureRules().length) ensureRules().push({ department_ids: [], job_titles: [], user_ids: [] })
  commit()
}
function ensureRules(): ACLRule[] {
  if (!draft.value.rules) draft.value.rules = []
  return draft.value.rules
}
function commit() { emit('update:modelValue', normalizeACL(draft.value)) }
</script>

<template>
  <div class="acl-editor">
    <el-radio-group :model-value="draft.scope" @update:model-value="changeScope">
      <el-radio-button value="all">公司全员</el-radio-button>
      <el-radio-button value="restricted">指定范围</el-radio-button>
    </el-radio-group>
    <div v-if="draft.scope === 'restricted'" class="acl-rule-list">
      <div class="acl-rule-help">同一规则内的条件需同时满足；多条规则之间满足任意一条即可查看。</div>
      <section v-for="(rule, index) in draft.rules" :key="index" class="acl-rule-card">
        <header><strong>规则 {{ index + 1 }}</strong><el-button v-if="(draft.rules?.length || 0) > 1" text type="danger" :icon="Delete" @click="removeRule(index)">删除</el-button></header>
        <el-form label-position="top">
          <el-form-item label="所属部门（可多选）">
            <el-select v-model="rule.department_ids" multiple filterable collapse-tags placeholder="不限部门" style="width: 100%" @change="commit">
              <el-option v-for="department in options.departments" :key="department.id" :label="department.path || department.name" :value="department.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="飞书职务（可多选）">
            <el-select v-model="rule.job_titles" multiple filterable allow-create default-first-option collapse-tags placeholder="不限职务" style="width: 100%" @change="commit">
              <el-option v-for="title in options.job_titles" :key="title" :label="title" :value="title" />
            </el-select>
          </el-form-item>
          <el-form-item label="指定人员（可多选）">
            <el-select v-model="rule.user_ids" multiple filterable collapse-tags placeholder="不指定个人" style="width: 100%" @change="commit">
              <el-option v-for="user in options.users" :key="user.id" :label="`${user.name}${user.job_title ? ` · ${user.job_title}` : ''}`" :value="user.id" />
            </el-select>
          </el-form-item>
        </el-form>
      </section>
      <el-button plain :icon="Plus" @click="addRule">添加另一组规则</el-button>
    </div>
  </div>
</template>
