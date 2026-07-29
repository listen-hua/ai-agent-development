package feishu

import (
	"context"
	"errors"
	"net/url"
)

type contactUserPayload struct {
	OpenID        string   `json:"open_id"`
	UserID        string   `json:"user_id"`
	Name          string   `json:"name"`
	DepartmentIDs []string `json:"department_ids"`
	JobTitle      string   `json:"job_title"`
	JobLevelID    string   `json:"job_level_id"`
	JobFamilyID   string   `json:"job_family_id"`
	EmployeeType  int      `json:"employee_type"`
	Avatar        struct {
		Avatar72 string `json:"avatar_72"`
	} `json:"avatar"`
	Status struct {
		IsFrozen   bool `json:"is_frozen"`
		IsResigned bool `json:"is_resigned"`
		IsExited   bool `json:"is_exited"`
	} `json:"status"`
}

type contactDepartmentPayload struct {
	Name               string `json:"name"`
	OpenDepartmentID   string `json:"open_department_id"`
	ParentDepartmentID string `json:"parent_department_id"`
	Order              string `json:"order"`
	MemberCount        int    `json:"member_count"`
	Status             struct {
		IsDeleted bool `json:"is_deleted"`
	} `json:"status"`
}

type contactDepartmentListOutput struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Items     []contactDepartmentPayload `json:"items"`
		HasMore   bool                       `json:"has_more"`
		PageToken string                     `json:"page_token"`
	} `json:"data"`
}

func (c *Client) ListContactDepartments(ctx context.Context) ([]ContactDepartment, error) {
	values, _, err := c.listContactDepartments(ctx)
	return values, err
}

func (c *Client) listContactDepartments(ctx context.Context) ([]ContactDepartment, bool, error) {
	if !c.Configured() {
		return nil, false, errors.New("feishu is not configured")
	}
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return nil, false, err
	}
	values := make([]ContactDepartment, 0)
	seen := map[string]bool{}
	rootIDs := make([]string, 0)
	rootAccessible := false
	pageToken := ""
	for page := 0; page < 100; page++ {
		query := url.Values{
			"department_id_type": {"open_department_id"},
			"user_id_type":       {"open_id"},
			"fetch_child":        {"false"},
			"page_size":          {"50"},
		}
		if pageToken != "" {
			query.Set("page_token", pageToken)
		}
		var output contactDepartmentListOutput
		endpoint := c.baseURL + "/open-apis/contact/v3/departments?" + query.Encode()
		if err = c.getJSON(ctx, endpoint, token, &output); err != nil {
			return nil, false, err
		}
		if output.Code != 0 {
			return nil, false, &APIError{Code: output.Code, Message: output.Msg}
		}
		for _, item := range output.Data.Items {
			if item.OpenDepartmentID == "" || item.Status.IsDeleted {
				continue
			}
			rootIDs = append(rootIDs, item.OpenDepartmentID)
			if item.OpenDepartmentID == "0" {
				rootAccessible = true
				continue
			}
			appendContactDepartment(&values, seen, item)
		}
		if !output.Data.HasMore || output.Data.PageToken == "" {
			break
		}
		pageToken = output.Data.PageToken
	}
	for _, rootID := range rootIDs {
		children, listErr := c.listContactDepartmentChildren(ctx, token, rootID)
		if listErr != nil {
			return nil, false, listErr
		}
		for _, child := range children {
			appendContactDepartment(&values, seen, child)
		}
	}
	return values, rootAccessible, nil
}

func (c *Client) listContactDepartmentChildren(ctx context.Context, token, departmentID string) ([]contactDepartmentPayload, error) {
	values := make([]contactDepartmentPayload, 0)
	pageToken := ""
	for page := 0; page < 100; page++ {
		query := url.Values{
			"department_id_type": {"open_department_id"},
			"user_id_type":       {"open_id"},
			"fetch_child":        {"true"},
			"page_size":          {"50"},
		}
		if pageToken != "" {
			query.Set("page_token", pageToken)
		}
		var output contactDepartmentListOutput
		endpoint := c.baseURL + "/open-apis/contact/v3/departments/" + url.PathEscape(departmentID) + "/children?" + query.Encode()
		if err := c.getJSON(ctx, endpoint, token, &output); err != nil {
			return nil, err
		}
		if output.Code != 0 {
			return nil, &APIError{Code: output.Code, Message: output.Msg}
		}
		values = append(values, output.Data.Items...)
		if !output.Data.HasMore || output.Data.PageToken == "" {
			break
		}
		pageToken = output.Data.PageToken
	}
	return values, nil
}

func appendContactDepartment(values *[]ContactDepartment, seen map[string]bool, item contactDepartmentPayload) {
	if item.OpenDepartmentID == "" || item.OpenDepartmentID == "0" || item.Status.IsDeleted || seen[item.OpenDepartmentID] {
		return
	}
	seen[item.OpenDepartmentID] = true
	*values = append(*values, ContactDepartment{
		OpenDepartmentID:   item.OpenDepartmentID,
		Name:               item.Name,
		ParentDepartmentID: item.ParentDepartmentID,
		Order:              item.Order,
		MemberCount:        item.MemberCount,
	})
}

func (c *Client) ListContactUsers(ctx context.Context) ([]ContactUser, error) {
	if !c.Configured() {
		return nil, errors.New("feishu is not configured")
	}
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return nil, err
	}
	departments, rootAccessible, err := c.listContactDepartments(ctx)
	if err != nil {
		return nil, err
	}
	departmentIDs := make([]string, 0, len(departments)+1)
	if rootAccessible {
		departmentIDs = append(departmentIDs, "0")
	}
	for _, department := range departments {
		departmentIDs = append(departmentIDs, department.OpenDepartmentID)
	}
	seen := map[string]bool{}
	values := []ContactUser{}
	for _, departmentID := range departmentIDs {
		pageToken := ""
		for page := 0; page < 100; page++ {
			query := url.Values{"department_id": {departmentID}, "department_id_type": {"open_department_id"}, "user_id_type": {"open_id"}, "page_size": {"50"}}
			if pageToken != "" {
				query.Set("page_token", pageToken)
			}
			var output struct {
				Code int    `json:"code"`
				Msg  string `json:"msg"`
				Data struct {
					Items     []contactUserPayload `json:"items"`
					HasMore   bool                 `json:"has_more"`
					PageToken string               `json:"page_token"`
				} `json:"data"`
			}
			if err = c.getJSON(ctx, c.baseURL+"/open-apis/contact/v3/users/find_by_department?"+query.Encode(), token, &output); err != nil {
				return nil, err
			}
			if output.Code != 0 {
				return nil, &APIError{Code: output.Code, Message: output.Msg}
			}
			for _, item := range output.Data.Items {
				if item.OpenID == "" || seen[item.OpenID] {
					continue
				}
				seen[item.OpenID] = true
				status := "active"
				if item.Status.IsFrozen {
					status = "frozen"
				}
				if item.Status.IsResigned || item.Status.IsExited {
					status = "inactive"
				}
				values = append(values, ContactUser{OpenID: item.OpenID, UserID: item.UserID, Name: item.Name, AvatarURL: item.Avatar.Avatar72, DepartmentIDs: item.DepartmentIDs, JobTitle: item.JobTitle, JobLevelID: item.JobLevelID, JobFamilyID: item.JobFamilyID, EmployeeType: item.EmployeeType, Status: status})
			}
			if !output.Data.HasMore || output.Data.PageToken == "" {
				break
			}
			pageToken = output.Data.PageToken
		}
	}
	return values, nil
}
