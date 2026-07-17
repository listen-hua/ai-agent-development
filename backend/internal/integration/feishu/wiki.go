package feishu

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

type WikiNode struct {
	SpaceID         string `json:"space_id"`
	NodeToken       string `json:"node_token"`
	ObjToken        string `json:"obj_token"`
	ObjType         string `json:"obj_type"`
	ParentNodeToken string `json:"parent_node_token"`
	NodeType        string `json:"node_type"`
	OriginNodeToken string `json:"origin_node_token"`
	OriginSpaceID   string `json:"origin_space_id"`
	HasChild        bool   `json:"has_child"`
	Title           string `json:"title"`
}

func ExtractWikiNodeToken(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("飞书知识库页面链接或节点 Token 不能为空")
	}
	if !strings.Contains(value, "://") {
		if strings.ContainsAny(value, "/?#") {
			return "", errors.New("飞书知识库节点 Token 格式无效")
		}
		return value, nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", errors.New("飞书知识库页面链接格式无效")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "wiki" && parts[i+1] != "" {
			return parts[i+1], nil
		}
	}
	return "", errors.New("请输入包含 /wiki/ 的飞书知识库页面链接或节点 Token")
}

func (c *Client) GetWikiNode(ctx context.Context, nodeToken string) (WikiNode, error) {
	if !c.Configured() {
		return WikiNode{}, errors.New("feishu is not configured")
	}
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return WikiNode{}, err
	}
	query := url.Values{"token": []string{nodeToken}, "obj_type": []string{"wiki"}}
	var output struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Node WikiNode `json:"node"`
		} `json:"data"`
	}
	endpoint := c.baseURL + "/open-apis/wiki/v2/spaces/get_node?" + query.Encode()
	if err = c.getJSON(ctx, endpoint, token, &output); err != nil {
		return WikiNode{}, err
	}
	if output.Code != 0 {
		return WikiNode{}, &APIError{Code: output.Code, Message: output.Msg}
	}
	if output.Data.Node.NodeToken == "" {
		return WikiNode{}, errors.New("飞书知识库节点不存在或应用无权访问")
	}
	return output.Data.Node, nil
}

func (c *Client) ListWikiNodes(ctx context.Context, spaceID, parentNodeToken string) ([]WikiNode, error) {
	if !c.Configured() {
		return nil, errors.New("feishu is not configured")
	}
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]WikiNode, 0)
	pageToken := ""
	for page := 0; page < 200; page++ {
		query := url.Values{"page_size": []string{"50"}}
		if parentNodeToken != "" {
			query.Set("parent_node_token", parentNodeToken)
		}
		if pageToken != "" {
			query.Set("page_token", pageToken)
		}
		var output struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Items     []WikiNode `json:"items"`
				PageToken string     `json:"page_token"`
				HasMore   bool       `json:"has_more"`
			} `json:"data"`
		}
		endpoint := c.baseURL + "/open-apis/wiki/v2/spaces/" + url.PathEscape(spaceID) + "/nodes?" + query.Encode()
		if err = c.getJSON(ctx, endpoint, token, &output); err != nil {
			return nil, err
		}
		if output.Code != 0 {
			return nil, &APIError{Code: output.Code, Message: output.Msg}
		}
		items = append(items, output.Data.Items...)
		if len(items) > 10000 {
			return nil, errors.New("飞书知识库单个父节点包含超过 10,000 个子节点")
		}
		pageToken = output.Data.PageToken
		if !output.Data.HasMore || pageToken == "" {
			return items, nil
		}
	}
	return nil, errors.New("飞书知识库分页超过安全限制")
}
