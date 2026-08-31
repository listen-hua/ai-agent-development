import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ImagePromptActionAdminPanel from './ImagePromptActionAdminPanel.vue'
import type { ImagePromptAction, ImagePromptActionInput } from '@/types/image-agent'

const messages = vi.hoisted(() => ({ warning: vi.fn(), success: vi.fn() }))
vi.mock('element-plus', async (importOriginal) => ({
  ...await importOriginal<typeof import('element-plus')>(),
  ElMessage: messages,
}))

function savedAction(input: ImagePromptActionInput): ImagePromptAction {
  return {
    id: 'action-multi-project',
    action_key: input.action_key,
    name: input.action_key,
    prompt_template: input.prompt_template,
    project_ids: input.project_ids,
    enabled: input.enabled,
    sort_order: input.sort_order,
    has_preview: false,
    created_at: '2026-08-11T00:00:00Z',
    updated_at: '2026-08-11T00:00:00Z',
  }
}

describe('ImagePromptActionAdminPanel', () => {
  it('submits all selected projects as the action scope', async () => {
    const saveAction = vi.fn(async (_id, input: ImagePromptActionInput) => ({ action: savedAction(input) }))
    const wrapper = mount(ImagePromptActionAdminPanel, {
      props: {
        actions: [],
        projects: [
          { id: 'project-a', project_key: 'a', name: '项目 A', description: '', acl: { scope: 'all' }, enabled: true, created_at: '', updated_at: '' },
          { id: 'project-b', project_key: 'b', name: '项目 B', description: '', acl: { scope: 'all' }, enabled: true, created_at: '', updated_at: '' },
        ],
        loading: false,
        saving: false,
        saveAction,
      },
      global: {
        stubs: {
          PromptActionPreviewPicker: true,
          ElButton: { template: '<button><slot /></button>' },
          ElDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          ElForm: { template: '<form><slot /></form>' },
          ElFormItem: { template: '<label><slot /></label>' },
          ElInput: true,
          ElInputNumber: true,
          ElOption: true,
          ElSelect: true,
          ElSwitch: true,
          ElTag: true,
          ElTable: true,
          ElTableColumn: true,
        },
        directives: { loading: () => undefined },
      },
    })
    const vm = wrapper.vm as unknown as {
      open: () => void
      save: () => Promise<void>
      form: ImagePromptActionInput
    }
    vm.open()
    vm.form.action_key = '共享风格'
    vm.form.prompt_template = '为多个项目使用同一套风格'
    vm.form.project_ids = ['project-a', 'project-b']

    await vm.save()

    expect(saveAction).toHaveBeenCalledOnce()
    expect(saveAction.mock.calls[0]?.[1]).toMatchObject({
      action_key: '共享风格',
      project_ids: ['project-a', 'project-b'],
    })
  })

})
