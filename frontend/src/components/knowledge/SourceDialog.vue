<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import ACLEditor from './ACLEditor.vue'
import type { ACL, ConnectedKnowledgeSourceType, DirectoryOptions } from '@/types/domain'
import { isACLValid } from '@/utils/acl'
import { inferFeishuSourceType } from '@/utils/feishuSource'

const props = defineProps<{ modelValue: boolean; options: DirectoryOptions; saving?: boolean; error?: string }>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  save: [value: { name: string; type: ConnectedKnowledgeSourceType; remote_token: string; default_acl: ACL }]
}>()

const form = reactive<{ name: string; type: ConnectedKnowledgeSourceType; remote_token: string }>({
  name: '',
  type: 'feishu_folder',
  remote_token: '',
})
const acl = ref<ACL>({ scope: 'all' })
const canSubmit = computed(() => Boolean(form.name.trim() && form.remote_token.trim() && isACLValid(acl.value)))
const isWiki = computed(() => form.type === 'feishu_wiki')

watch(
  () => props.modelValue,
  (open) => {
    if (!open) return
    Object.assign(form, { name: '', type: 'feishu_folder', remote_token: '' })
    acl.value = { scope: 'all' }
  },
)

watch(
  () => form.remote_token,
  (value) => {
    const inferred = inferFeishuSourceType(value)
    if (inferred) form.type = inferred
  },
)

function submit() {
  if (!canSubmit.value) return
  emit('save', {
    name: form.name.trim(),
    type: form.type,
    remote_token: form.remote_token.trim(),
    default_acl: acl.value,
  })
}

function requestClose(value: boolean) {
  if (props.saving && !value) return
  emit('update:modelValue', value)
}
</script>

<template>
  <el-dialog
    :model-value="modelValue"
    title="连接飞书资料源"
    width="min(680px, 94vw)"
	    :close-on-click-modal="!saving"
	    :close-on-press-escape="!saving"
	    @update:model-value="requestClose"
  >
    <el-alert type="info" :closable="false" show-icon class="source-guide">
      <template #title>
        连接后会递归读取{{ isWiki ? '当前知识库页面及其子页面' : '文件夹及其子文件夹' }}，并每 15 分钟增量同步
      </template>
      首次同步和文件变更只生成草稿版本，知识管理员发布后才会用于员工问答。
    </el-alert>
	    <el-alert v-if="error" type="error" :closable="false" show-icon class="source-error-alert" :title="error" />
    <el-form label-position="top">
      <el-form-item label="资料源类型">
        <el-radio-group v-model="form.type">
          <el-radio-button value="feishu_folder" @click="form.type = 'feishu_folder'">云空间文件夹</el-radio-button>
          <el-radio-button value="feishu_wiki" @click="form.type = 'feishu_wiki'">飞书知识库</el-radio-button>
        </el-radio-group>
      </el-form-item>
      <el-form-item label="资料源名称">
        <el-input v-model="form.name" :placeholder="isWiki ? '例如：公司行政知识库' : '例如：行政制度文件夹'" />
      </el-form-item>
      <el-form-item :label="isWiki ? '知识库页面链接或节点 Token' : '飞书文件夹链接或 Token'">
        <el-input
          v-model="form.remote_token"
          :placeholder="isWiki ? 'https://公司域名.feishu.cn/wiki/wikcn... 或 wikcn...' : 'https://公司域名.feishu.cn/drive/folder/fldcn... 或 fldcn...'"
        />
        <small v-if="isWiki" class="form-tip">
          请把应用机器人所在群加入知识空间成员，或把该页面及其子页面共享给该群，并授予可阅读权限。
        </small>
        <small v-else class="form-tip">
          请新建群聊、把本应用添加为群机器人，再把文件夹共享给该群并授予可查看、可下载权限。
        </small>
      </el-form-item>
      <el-form-item label="默认可见范围">
        <ACLEditor v-model="acl" :options="options" />
        <small class="form-tip">同步进来的新文档继承此范围；之后仍可逐个调整文档权限。</small>
      </el-form-item>
    </el-form>
    <template #footer>
	      <el-button :disabled="saving" @click="requestClose(false)">取消</el-button>
	      <el-button type="primary" :loading="saving" :disabled="!canSubmit" @click="submit">连接并开始同步</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.source-guide { margin-bottom: 18px; }
.source-error-alert { margin-bottom: 18px; }
</style>
