package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shillcollin/gai/core"
	"github.com/shillcollin/gai/tools"
)

type tavilyClient struct {
	apiKey     string
	httpClient *http.Client
}

func newTavilyClient(client *http.Client, apiKey string) *tavilyClient {
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	apiKey = strings.TrimSpace(apiKey)
	return &tavilyClient{apiKey: apiKey, httpClient: client}
}

func (tc *tavilyClient) enabled() bool {
	return tc != nil && tc.apiKey != ""
}

func (tc *tavilyClient) searchTool() core.ToolHandle {
	if !tc.enabled() {
		return nil
	}

	type searchInput struct {
		Query             string `json:"query"`
		SearchDepth       string `json:"search_depth,omitempty"`
		MaxResults        int    `json:"max_results,omitempty"`
		IncludeRawContent bool   `json:"include_raw_content,omitempty"`
		IncludeAnswer     bool   `json:"include_answer,omitempty"`
	}

	type searchImage struct {
		URL         string `json:"url"`
		Description string `json:"description,omitempty"`
	}

	type searchResult struct {
		Title      string          `json:"title"`
		URL        string          `json:"url"`
		Content    string          `json:"content,omitempty"`
		Score      float64         `json:"score,omitempty"`
		RawContent json.RawMessage `json:"raw_content,omitempty"`
		Favicon    string          `json:"favicon,omitempty"`
	}

	type searchOutput struct {
		Query          string         `json:"query"`
		Answer         string         `json:"answer,omitempty"`
		Results        []searchResult `json:"results"`
		Images         []searchImage  `json:"images,omitempty"`
		AutoParameters map[string]any `json:"auto_parameters,omitempty"`
		ResponseTime   string         `json:"response_time,omitempty"`
		RequestID      string         `json:"request_id,omitempty"`
	}

	tool := tools.New[searchInput, searchOutput](
		"web_search",
		"Search the public web with Tavily; follow up with url_extract via Tavily Extract when you need the full document. Optional fields: search_depth ('basic' or 'advanced', default 'advanced'), max_results (1-10, default 5), include_raw_content (bool), include_answer (bool).",
		func(ctx context.Context, in searchInput, meta core.ToolMeta) (searchOutput, error) {
			if strings.TrimSpace(in.Query) == "" {
				return searchOutput{}, errors.New("query is required")
			}

			body := map[string]any{
				"query": strings.TrimSpace(in.Query),
			}
			depth := strings.ToLower(strings.TrimSpace(in.SearchDepth))
			if depth == "" {
				depth = "advanced"
			}
			if depth != "basic" && depth != "advanced" {
				return searchOutput{}, fmt.Errorf("invalid search_depth %q", in.SearchDepth)
			}
			body["search_depth"] = depth

			maxResults := in.MaxResults
			if maxResults <= 0 {
				maxResults = 5
			}
			if maxResults > 10 {
				maxResults = 10
			}
			body["max_results"] = maxResults

			if in.IncludeAnswer {
				body["include_answer"] = true
			}
			if in.IncludeRawContent {
				body["include_raw_content"] = true
			}

			var resp struct {
				Query   string        `json:"query"`
				Answer  string        `json:"answer"`
				Images  []searchImage `json:"images"`
				Results []struct {
					Title      string          `json:"title"`
					URL        string          `json:"url"`
					Content    string          `json:"content"`
					Score      float64         `json:"score"`
					RawContent json.RawMessage `json:"raw_content"`
					Favicon    string          `json:"favicon"`
				} `json:"results"`
				AutoParameters map[string]any  `json:"auto_parameters"`
				ResponseTime   json.RawMessage `json:"response_time"`
				RequestID      string          `json:"request_id"`
			}

			if err := tc.do(ctx, http.MethodPost, "https://api.tavily.com/search", body, &resp); err != nil {
				return searchOutput{}, err
			}

			out := searchOutput{
				Query:          resp.Query,
				Answer:         strings.TrimSpace(resp.Answer),
				AutoParameters: resp.AutoParameters,
				Images:         make([]searchImage, 0, len(resp.Images)),
				Results:        make([]searchResult, 0, len(resp.Results)),
				RequestID:      resp.RequestID,
			}

			if len(resp.ResponseTime) > 0 {
				out.ResponseTime = strings.Trim(string(resp.ResponseTime), "\"")
			}

			if len(resp.Images) > 0 {
				out.Images = append(out.Images, resp.Images...)
			}

			for _, item := range resp.Results {
				out.Results = append(out.Results, searchResult{
					Title:      item.Title,
					URL:        item.URL,
					Content:    item.Content,
					Score:      item.Score,
					RawContent: item.RawContent,
					Favicon:    item.Favicon,
				})
			}

			return out, nil
		},
	)

	return tools.NewCoreAdapter(tool)
}

func (tc *tavilyClient) extractTool() core.ToolHandle {
	if !tc.enabled() {
		return nil
	}

	type extractInput struct {
		URL string `json:"url"`
	}

	type extractResult struct {
		URL        string          `json:"url"`
		RawContent json.RawMessage `json:"raw_content,omitempty"`
		Images     []struct {
			URL         string `json:"url"`
			Description string `json:"description,omitempty"`
		} `json:"images,omitempty"`
		Favicon string `json:"favicon,omitempty"`
	}

	type extractOutput struct {
		Results      []extractResult  `json:"results"`
		Failed       []map[string]any `json:"failed_results,omitempty"`
		ResponseTime json.RawMessage  `json:"response_time,omitempty"`
		RequestID    string           `json:"request_id,omitempty"`
	}

	tool := tools.New[extractInput, extractOutput](
		"url_extract",
		"Fetch full page content with Tavily Extract; use this after web_search finds promising sources.",
		func(ctx context.Context, in extractInput, meta core.ToolMeta) (extractOutput, error) {
			url := strings.TrimSpace(in.URL)
			if url == "" {
				return extractOutput{}, errors.New("url is required")
			}

			payload := map[string]any{"urls": url}

			var resp struct {
				Results      []extractResult  `json:"results"`
				Failed       []map[string]any `json:"failed_results"`
				ResponseTime json.RawMessage  `json:"response_time"`
				RequestID    string           `json:"request_id"`
			}

			if err := tc.do(ctx, http.MethodPost, "https://api.tavily.com/extract", payload, &resp); err != nil {
				return extractOutput{}, err
			}

			out := extractOutput{
				Results:      make([]extractResult, 0, len(resp.Results)),
				Failed:       resp.Failed,
				RequestID:    resp.RequestID,
				ResponseTime: resp.ResponseTime,
			}

			out.Results = append(out.Results, resp.Results...)
			return out, nil
		},
	)

	return tools.NewCoreAdapter(tool)
}

func (tc *tavilyClient) do(ctx context.Context, method, rawURL string, payload any, v any) error {
	if !tc.enabled() {
		return errors.New("tavily api key missing")
	}

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tc.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := tc.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return fmt.Errorf("tavily %s %s: %s", method, rawURL, strings.TrimSpace(string(data)))
	}
	if v == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(v)
}
