package domain

import "testing"

func TestACLRuleRequiresAllConfiguredDimensions(t *testing.T) {
	acl := ACL{Scope: "restricted", Rules: []ACLRule{{DepartmentIDs: []string{"dept_finance"}, JobTitles: []string{"会计"}}}}
	accountant := User{ID: "u1", DepartmentIDs: []string{"dept_finance"}, JobTitle: "会计", Roles: []Role{RoleEmployee}}
	financeManager := User{ID: "u2", DepartmentIDs: []string{"dept_finance"}, JobTitle: "财务经理", Roles: []Role{RoleEmployee}}
	otherAccountant := User{ID: "u3", DepartmentIDs: []string{"dept_branch"}, JobTitle: "会计", Roles: []Role{RoleEmployee}}
	if !acl.Allows(accountant) {
		t.Fatal("expected matching department and job title to be allowed")
	}
	if acl.Allows(financeManager) || acl.Allows(otherAccountant) {
		t.Fatal("a rule must use AND semantics across its configured dimensions")
	}
}

func TestACLRulesUseOrSemanticsAndAdministratorsDoNotBypass(t *testing.T) {
	acl := ACL{Scope: "restricted", Rules: []ACLRule{{UserIDs: []string{"special"}}, {JobTitles: []string{"法务"}}}}
	if !acl.Allows(User{ID: "special", Roles: []Role{RoleEmployee}}) || !acl.Allows(User{ID: "other", JobTitle: "法务", Roles: []Role{RoleEmployee}}) {
		t.Fatal("expected either rule to allow access")
	}
	if acl.Allows(User{ID: "auditor", Roles: []Role{RoleAuditor}}) {
		t.Fatal("auditors must not implicitly bypass document ACLs")
	}
	if acl.Allows(User{ID: "root", Roles: []Role{RoleSuperAdmin}, Permissions: AllPermissionKeys}) {
		t.Fatal("administrative permissions must not bypass document ACLs")
	}
}

func TestRestrictedACLRejectsEmptyRules(t *testing.T) {
	if err := (ACL{Scope: "restricted", Rules: []ACLRule{{}}}).Validate(); err == nil {
		t.Fatal("expected empty restricted rule to fail validation")
	}
}
