<script setup lang="ts">
import { DocumentChecked, EditPen, Lock, Promotion } from '@element-plus/icons-vue'
import type { KnowledgeDocument } from '@/types/domain'
import { aclSummary } from '@/utils/acl'

defineProps<{ documents: KnowledgeDocument[]; loading: boolean; publishingIds?: string[] }>()
const emit = defineEmits<{ publish: [id: string]; editAcl: [document: KnowledgeDocument] }>()

const statusMap: Record<string, { label: string; type: 'success' | 'warning' | 'info' | 'danger' }> = {
  published: { label: '已发布', type: 'success' },
  draft: { label: '待发布', type: 'warning' },
  archived: { label: '已归档', type: 'info' },
  unavailable: { label: '源文件不可用', type: 'danger' },
}

function hasDraft(document: KnowledgeDocument) {
  return document.versions.some((version) => version.status === 'draft')
}

function status(document: KnowledgeDocument) {
  if (document.status === 'published' && hasDraft(document)) {
    return { label: '新版本待发布', type: 'warning' as const }
  }
  return statusMap[document.status] || { label: document.status, type: 'info' as const }
}

function date(value: string) {
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value))
}
</script>

<template>
  <div class="data-table-wrap">
    <el-table :data="documents" v-loading="loading" row-key="id">
      <el-table-column label="制度文件" min-width="280">
        <template #default="{ row }">
          <div class="document-name">
            <span><DocumentChecked /></span>
            <div>
              <strong>{{ row.title }}</strong>
              <small>{{ row.versions[0]?.mime_type || '云文档' }}</small>
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="最新版本" width="110">
        <template #default="{ row }">v{{ row.versions[0]?.version }}</template>
      </el-table-column>
      <el-table-column label="可见范围" width="150">
        <template #default="{ row }"><span class="acl-cell"><Lock />{{ aclSummary(row.acl) }}</span></template>
      </el-table-column>
      <el-table-column label="状态" width="140">
        <template #default="{ row }">
          <el-tag :type="status(row).type" effect="light" round>{{ status(row).label }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="更新时间" width="150">
        <template #default="{ row }">{{ date(row.updated_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" align="right" width="210" fixed="right">
        <template #default="{ row }">
          <el-button size="small" text :icon="EditPen" @click="emit('editAcl', row)">权限</el-button>
          <el-button
            v-if="hasDraft(row) && row.status !== 'unavailable'"
            size="small"
            type="primary"
            :icon="Promotion"
            :loading="publishingIds?.includes(row.id)"
            @click="emit('publish', row.id)"
          >
            发布新版本
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>
