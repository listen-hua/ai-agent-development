import { mount, flushPromises } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ImageProjectAdminPanel from './ImageProjectAdminPanel.vue'

const messages = vi.hoisted(() => ({ warning: vi.fn() }))
vi.mock('element-plus', async (importOriginal) => ({
  ...await importOriginal<typeof import('element-plus')>(),
  ElMessage: messages,
}))

function mountPanel(saveProject = vi.fn()) {
  return mount(ImageProjectAdminPanel, {
    props: {
      projects: [],
      directory: { departments: [], job_titles: [], users: [] },
      loading: false,
      saving: false,
      saveProject,
    },
    global: {
      stubs: {
        ACLEditor: true,
        ElButton: { template: '<button><slot /></button>' },
        ElDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        ElForm: { template: '<form><slot /></form>' },
        ElFormItem: { template: '<label><slot /></label>' },
        ElInput: true,
        ElSwitch: true,
        ElTag: true,
        ElTable: true,
        ElTableColumn: true,
      },
      directives: { loading: () => undefined },
    },
  })
}

describe('ImageProjectAdminPanel', () => {
  it('keeps the editor open and shows validation feedback for an invalid key', async () => {
    messages.warning.mockClear()
    const saveProject = vi.fn()
    const wrapper = mountPanel(saveProject)
    const vm = wrapper.vm as unknown as { open: () => void; save: () => Promise<void>; form: { project_key: string; name: string }; dialog: boolean }
    vm.open()
    vm.form.project_key = '1中文项目'
    vm.form.name = '活动图'
    await vm.save()
    expect(saveProject).not.toHaveBeenCalled()
    expect(messages.warning).toHaveBeenCalledOnce()
    expect(vm.dialog).toBe(true)
  })

  it('closes only after the project request succeeds', async () => {
    let rejectRequest!: (reason?: unknown) => void
    const request = new Promise((_resolve, reject) => { rejectRequest = reject })
    const saveProject = vi.fn(() => request)
    const wrapper = mountPanel(saveProject)
    const vm = wrapper.vm as unknown as { open: () => void; save: () => Promise<void>; form: { project_key: string; name: string }; dialog: boolean }
    vm.open()
    vm.form.project_key = '宣传图项目'
    vm.form.name = '活动图'
    const saving = vm.save()
    expect(vm.dialog).toBe(true)
    rejectRequest(new Error('request failed'))
    await saving
    await flushPromises()
    expect(vm.dialog).toBe(true)

    await wrapper.setProps({ saveProject: vi.fn().mockResolvedValue({ id: 'project-1' }) })
    await vm.save()
    expect(vm.dialog).toBe(false)
  })
})
