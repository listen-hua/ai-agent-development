<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { Connection, Promotion, Upload } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import PageHeader from '@/components/common/PageHeader.vue'
import MetricCard from '@/components/common/MetricCard.vue'
import ACLDialog from '@/components/knowledge/ACLDialog.vue'
import DocumentTable from '@/components/knowledge/DocumentTable.vue'
import SourceDialog from '@/components/knowledge/SourceDialog.vue'
import SourceStrip from '@/components/knowledge/SourceStrip.vue'
import UploadDialog from '@/components/knowledge/UploadDialog.vue'
import { directoryService, knowledgeService } from '@/services/admin'
import { ApiError } from '@/services/api'
import type { ACL, ConnectedKnowledgeSourceType, DirectoryOptions, KnowledgeDocument, KnowledgeSource } from '@/types/domain'

const emptyOptions: DirectoryOptions = { departments: [], job_titles: [], users: [] }
const documents = ref<KnowledgeDocument[]>([])
const sources = ref<KnowledgeSource[]>([])
const options = ref<DirectoryOptions>(emptyOptions)
const loading = ref(false)
const uploading = ref(false)
const savingACL = ref(false)
const connectingSource = ref(false)
const publishingIds = ref<string[]>([])
const batchPublishing = ref(false)
const sourceError = ref('')
const sourceOpen = ref(false)
const uploadOpen = ref(false)
const aclOpen = ref(false)
const selectedDocument = ref<KnowledgeDocument>()
const pollingSources = new Set<string>()
let disposed = false

const publishedCount = computed(() => documents.value.filter((item) => item.status === 'published').length)
const pendingCount = computed(() => documents.value.filter((item) => item.versions.some((version) => version.status === 'draft')).length)
const pendingDocuments = computed(() => documents.value.filter((item) => item.status !== 'unavailable' && item.versions.some((version) => version.status === 'draft')))

async function load() {
  loading.value = true
  try {
    const [documentList, sourceList, directoryOptions] = await Promise.all([
      knowledgeService.listDocuments(),
      knowledgeService.listSources(),
      directoryService.options(),
    ])
    documents.value = documentList
    sources.value = sourceList
    options.value = directoryOptions
  } finally {
    loading.value = false
  }
}

async function refreshKnowledge() {
  const [documentList, sourceList] = await Promise.all([
    knowledgeService.listDocuments(),
    knowledgeService.listSources(),
  ])
  documents.value = documentList
  sources.value = sourceList
}

async function pollSource(sourceId: string) {
  if (pollingSources.has(sourceId)) return
  pollingSources.add(sourceId)
  try {
    for (let attempt = 0; attempt < 150 && !disposed; attempt += 1) {
      await new Promise((resolve) => window.setTimeout(resolve, 2000))
      if (disposed) return
      await refreshKnowledge()
      const source = sources.value.find((item) => item.id === sourceId)
      if (!source || !['queued', 'syncing'].includes(source.sync_status)) {
        if (source?.sync_status === 'success') ElMessage.success('飞书资料源同步完成，新内容已生成待发布版本')
        if (source?.sync_status === 'partial') ElMessage.warning(`同步完成，但有 ${source.last_sync_stats.failed} 个文件失败`)
        if (source?.sync_status === 'error') ElMessage.error(source.sync_error || '飞书资料源同步失败')
        return
      }
    }
  } finally {
    pollingSources.delete(sourceId)
  }
}

async function createSource(input: { name: string; type: ConnectedKnowledgeSourceType; remote_token: string; default_acl: ACL }) {
	  connectingSource.value = true
	  sourceError.value = ''
	  try {
	    const source = await knowledgeService.createSource(input)
	    sourceOpen.value = false
	    await knowledgeService.syncSource(source.id)
	    ElMessage.success('资料源已连接，正在进行首次同步')
	    await refreshKnowledge()
	    void pollSource(source.id)
	  } catch (error) {
	    const message = sourceErrorMessage(error)
	    sourceError.value = message
	    ElMessage.error(message)
	  } finally {
	    connectingSource.value = false
	  }
}

function sourceErrorMessage(error: unknown) {
	  if (error instanceof ApiError) {
	    const detail = error.detail?.trim()
	    return detail || error.message
	  }
	  return error instanceof Error ? error.message : '连接飞书资料源失败'
}

function openSourceDialog() {
	  sourceError.value = ''
	  sourceOpen.value = true
}

async function upload(file: File, acl: ACL) {
  uploading.value = true
  try {
    await knowledgeService.upload(file, acl)
    uploadOpen.value = false
    ElMessage.success('文件已解析为草稿，确认后再发布')
    await refreshKnowledge()
  } finally {
    uploading.value = false
  }
}

async function publish(id: string) {
  if (publishingIds.value.includes(id)) return
  try {
    await ElMessageBox.confirm('发布后，新版本将参与员工问答检索。确认发布？', '发布制度版本', { type: 'warning' })
  } catch {
    return
  }
  publishingIds.value = [...publishingIds.value, id]
  try {
    await knowledgeService.publish(id)
    ElMessage.success('制度版本已发布')
    await refreshKnowledge()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '制度版本发布失败')
  } finally {
    publishingIds.value = publishingIds.value.filter((item) => item !== id)
  }
}

async function publishAll() {
  const ids = pendingDocuments.value.map((item) => item.id).filter((id) => !publishingIds.value.includes(id))
  if (!ids.length || batchPublishing.value) return
  try {
    await ElMessageBox.confirm(
      `将发布 ${ids.length} 份待发布制度，发布后会立即参与员工问答检索。确认继续？`,
      '批量发布制度',
      { type: 'warning' },
    )
  } catch {
    return
  }
  batchPublishing.value = true
  publishingIds.value = [...new Set([...publishingIds.value, ...ids])]
  try {
    const results = await Promise.allSettled(ids.map((id) => knowledgeService.publish(id)))
    const failed = results.filter((result) => result.status === 'rejected').length
    await refreshKnowledge()
    if (failed) {
      ElMessage.warning(`已发布 ${ids.length - failed} 份，${failed} 份发布失败，请单独重试`)
    } else {
      ElMessage.success(`已发布 ${ids.length} 份制度`)
    }
  } finally {
    const completed = new Set(ids)
    publishingIds.value = publishingIds.value.filter((id) => !completed.has(id))
    batchPublishing.value = false
  }
}

async function sync(source: KnowledgeSource) {
  await knowledgeService.syncSource(source.id)
  ElMessage.success('已开始同步')
  await refreshKnowledge()
  void pollSource(source.id)
}

function editACL(document: KnowledgeDocument) {
  selectedDocument.value = document
  aclOpen.value = true
}

async function saveACL(acl: ACL) {
  if (!selectedDocument.value) return
  savingACL.value = true
  try {
    await knowledgeService.updateACL(selectedDocument.value.id, acl)
    aclOpen.value = false
    ElMessage.success('文档可见范围已更新')
    await refreshKnowledge()
  } finally {
    savingACL.value = false
  }
}

onMounted(load)
onBeforeUnmount(() => { disposed = true })
</script>

<template>
  <section class="admin-page">
    <PageHeader eyebrow="KNOWLEDGE GOVERNANCE" title="制度知识库" description="资料先解析为草稿，经权限确认和命中测试后再发布给员工。">
	      <el-button :icon="Connection" @click="openSourceDialog">连接飞书</el-button>
      <el-button type="primary" :icon="Upload" @click="uploadOpen = true">上传制度</el-button>
    </PageHeader>
    <div class="metric-grid">
      <MetricCard label="制度文件" :value="documents.length" note="全部版本" />
      <MetricCard label="已发布" :value="publishedCount" note="参与线上检索" tone="green" />
      <MetricCard label="待确认" :value="pendingCount" note="不会影响线上回答" tone="amber" />
      <MetricCard label="飞书资料源" :value="sources.length" note="每 15 分钟增量同步" tone="violet" />
    </div>
	    <SourceStrip v-if="sources.length" :sources="sources" @add="openSourceDialog" @sync="sync" />
    <div class="section-heading">
      <div>
        <h2>制度文件</h2>
        <p>只有“已发布”的当前有效版本会参与普通问答；文档权限在检索进入模型前强制执行。</p>
      </div>
      <div class="section-actions">
        <el-button
          v-if="pendingDocuments.length"
          type="primary"
          :icon="Promotion"
          :loading="batchPublishing"
          @click="publishAll"
        >
          发布全部待发布（{{ pendingDocuments.length }}）
        </el-button>
        <el-input class="table-search" placeholder="搜索制度名称" clearable />
      </div>
    </div>
    <DocumentTable :documents="documents" :loading="loading" :publishing-ids="publishingIds" @publish="publish" @edit-acl="editACL" />
	    <SourceDialog v-model="sourceOpen" :options="options" :saving="connectingSource" :error="sourceError" @save="createSource" />
    <UploadDialog v-model="uploadOpen" :uploading="uploading" :options="options" @upload="upload" />
    <ACLDialog v-model="aclOpen" :document="selectedDocument" :options="options" :saving="savingACL" @save="saveACL" />
  </section>
</template>

<style scoped>
.section-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  flex-wrap: wrap;
}

@media (max-width: 760px) {
  .section-heading {
    align-items: flex-start;
    flex-direction: column;
    gap: 12px;
  }

  .section-actions {
    width: 100%;
    justify-content: flex-start;
  }

  .section-actions :deep(.el-button) {
    width: 100%;
    margin: 0;
  }
}
</style>
