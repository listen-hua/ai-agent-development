import type { ACL, ACLRule } from '@/types/domain'

export function cloneACL(value?: ACL): ACL {
  if (!value || value.scope === 'all') return { scope: 'all' }
  const rules = value.rules?.length ? value.rules : legacyRules(value)
  return { scope: 'restricted', rules: rules.map(cloneRule) }
}

export function normalizeACL(value: ACL): ACL {
  if (value.scope === 'all') return { scope: 'all' }
  return {
    scope: 'restricted',
    rules: (value.rules || []).map((rule) => ({
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
  return !!value.rules?.length && value.rules.every((rule) => ruleSize(rule) > 0)
}

export function aclSummary(value: ACL): string {
  if (value.scope === 'all') return '公司全员'
  const count = value.rules?.length || 0
  return count ? `${count} 组权限规则` : '未配置范围'
}

function legacyRules(value: ACL): ACLRule[] {
  const rule: ACLRule = { department_ids: value.department_ids, user_ids: value.user_ids }
  return ruleSize(rule) ? [rule] : [{ department_ids: [], job_titles: [], user_ids: [] }]
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

function ruleSize(rule: ACLRule): number {
  return (rule.department_ids?.length || 0) + (rule.job_titles?.length || 0) + (rule.job_level_ids?.length || 0) + (rule.job_family_ids?.length || 0) + (rule.employee_types?.length || 0) + (rule.user_ids?.length || 0)
}
