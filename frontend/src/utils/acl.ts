import type { ACL, ACLRule, PermissionKey, Role } from '@/types/domain'

export function cloneACL(value?: ACL): ACL {
  if (!value || value.scope === 'all') return { scope: 'all' }
  const rules = value.rules?.length ? value.rules : legacyRules(value)
  return {
    scope: 'restricted',
    rules: rules.map(cloneRule),
    permission_keys: uniquePermissions([...(value.permission_keys || []), ...legacyRolePermissions(value.role_names || [])]),
  }
}

export function normalizeACL(value: ACL): ACL {
  if (value.scope === 'all') return { scope: 'all' }
  return {
    scope: 'restricted',
    permission_keys: uniquePermissions(value.permission_keys),
    rules: (value.rules || []).filter((rule) => ruleSize(rule) > 0).map((rule) => ({
      department_ids: unique(rule.department_ids),
      job_titles: unique(rule.job_titles),
      job_level_ids: unique(rule.job_level_ids),
      job_family_ids: unique(rule.job_family_ids),
      employee_types: [...new Set(rule.employee_types || [])],
      user_ids: unique(rule.user_ids),
    })),
  }
}

export function isACLValid(value: ACL): boolean {
  if (value.scope === 'all') return true
  const validRules = (value.rules || []).filter((rule) => ruleSize(rule) > 0)
  return validRules.length > 0 || Boolean(value.permission_keys?.length)
}

export function aclSummary(value: ACL): string {
  if (value.scope === 'all') return '公司全员'
  const count = (value.rules || []).filter((rule) => ruleSize(rule) > 0).length
  const permissionCount = value.permission_keys?.length || 0
  if (count && permissionCount) return `${count} 组组织规则 + ${permissionCount} 项权限`
  if (count) return `${count} 组权限规则`
  if (permissionCount) return `${permissionCount} 项系统权限`
  return '未配置范围'
}

function legacyRules(value: ACL): ACLRule[] {
  const rule: ACLRule = { department_ids: value.department_ids, user_ids: value.user_ids }
  return ruleSize(rule) ? [rule] : []
}

function cloneRule(rule: ACLRule): ACLRule {
  return {
    department_ids: [...(rule.department_ids || [])],
    job_titles: [...(rule.job_titles || [])],
    job_level_ids: [...(rule.job_level_ids || [])],
    job_family_ids: [...(rule.job_family_ids || [])],
    employee_types: [...(rule.employee_types || [])],
    user_ids: [...(rule.user_ids || [])],
  }
}

function unique(values: string[] = []): string[] {
  return [...new Set(values.map((value) => value.trim()).filter(Boolean))]
}

function uniquePermissions(values: PermissionKey[] = []): PermissionKey[] {
  return [...new Set(values)]
}

function legacyRolePermissions(roles: Role[]): PermissionKey[] {
  const mapping: Partial<Record<Role, PermissionKey>> = {
    employee: 'agent_use',
    knowledge_admin: 'knowledge_manage',
    notification_admin: 'notification_manage',
    image_admin: 'image_manage',
    auditor: 'audit_view',
    super_admin: 'user_manage',
  }
  return roles.flatMap((role) => mapping[role] ? [mapping[role]!] : [])
}

function ruleSize(rule: ACLRule): number {
  return (rule.department_ids?.length || 0) + (rule.job_titles?.length || 0) + (rule.job_level_ids?.length || 0) + (rule.job_family_ids?.length || 0) + (rule.employee_types?.length || 0) + (rule.user_ids?.length || 0)
}
