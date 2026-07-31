import { describe, expect, it } from 'vitest'
import { aclSummary, cloneACL, isACLValid, normalizeACL } from './acl'

describe('ACL utilities', () => {
  it('requires every restricted rule to contain a condition', () => {
    expect(isACLValid({ scope: 'restricted', rules: [{ department_ids: [] }] })).toBe(false)
    expect(isACLValid({ scope: 'restricted', rules: [{ job_titles: ['会计'] }] })).toBe(true)
  })

  it('normalizes duplicate values without mutating the original', () => {
    const original = { scope: 'restricted' as const, rules: [{ job_titles: ['会计', '会计'] }] }
    const cloned = cloneACL(original)
    const normalized = normalizeACL(cloned)
    expect(normalized.rules?.[0].job_titles).toEqual(['会计'])
    expect(original.rules[0].job_titles).toHaveLength(2)
    expect(aclSummary(normalized)).toBe('1 组权限规则')
  })

  it('preserves and validates explicit permission-key ACLs', () => {
    const acl = cloneACL({ scope: 'restricted', role_names: ['knowledge_admin'] })
    expect(acl.permission_keys).toEqual(['knowledge_manage'])
    expect(isACLValid(acl)).toBe(true)
    expect(normalizeACL(acl).rules).toEqual([])
  })
})
