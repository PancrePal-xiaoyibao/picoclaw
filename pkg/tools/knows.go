package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	knowsAllowedDataScopes = map[string]struct{}{
		"PAPER":    {},
		"PAPER_CN": {},
		"GUIDE":    {},
		"MEETING":  {},
	}

	knowsAllowedAnswerTypes = map[string]struct{}{
		"CLINICAL":        {},
		"RESEARCH":        {},
		"POPULAR_SCIENCE": {},
	}

	knowsAllDataScopes = []string{"PAPER", "PAPER_CN", "GUIDE", "MEETING"}
)

const (
	defaultKnowsRequestTimeout = 120 * time.Second
	defaultKnowsMaxRetries     = 3
	defaultKnowsRetryBackoff   = 500 * time.Millisecond
	defaultKnowsBatchLimit     = 5
	defaultKnowsCacheTTL       = time.Hour
	defaultKnowsCacheEntries   = 500
)

type KnowsToolOptions struct {
	APIKey           string
	APIBaseURL       string
	DefaultDataScope []string
	RequestTimeout   time.Duration
	MaxRetries       int
	RetryBackoff     time.Duration
	BatchConcurrency int
	CacheTTL         time.Duration
	CacheMaxEntries  int
}

type knowsClient struct {
	baseURL      string
	apiKey       string
	httpClient   *http.Client
	maxRetries   int
	retryBackoff time.Duration
	cache        *knowsDetailCache
}

type knowsDetailCache struct {
	mu         sync.Mutex
	entries    map[string]knowsCacheEntry
	order      []string
	ttl        time.Duration
	maxEntries int
}

type knowsCacheEntry struct {
	value     interface{}
	expiresAt time.Time
}

type knowsTool struct {
	name        string
	description string
	parameters  map[string]interface{}
	handler     func(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

type knowsBatchAnswerRequest struct {
	QuestionID string
	AnswerType string
}

type knowsBatchEvidenceRequest struct {
	EvidenceID string
	Type       string
}

func NewKnowsTools(opts KnowsToolOptions) ([]Tool, error) {
	apiKey := strings.TrimSpace(opts.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("knows api_key is required")
	}

	apiBaseURL := strings.TrimSpace(opts.APIBaseURL)
	if apiBaseURL == "" {
		return nil, fmt.Errorf("knows api_base_url is required")
	}

	defaultScope, err := normalizeDataScopes(opts.DefaultDataScope)
	if err != nil {
		return nil, fmt.Errorf("invalid knows default_data_scope: %w", err)
	}
	if len(defaultScope) == 0 {
		defaultScope = append([]string(nil), knowsAllDataScopes...)
	}

	timeout := opts.RequestTimeout
	if timeout <= 0 {
		timeout = defaultKnowsRequestTimeout
	}

	maxRetries := opts.MaxRetries
	if maxRetries < 0 {
		maxRetries = defaultKnowsMaxRetries
	}

	retryBackoff := opts.RetryBackoff
	if retryBackoff <= 0 {
		retryBackoff = defaultKnowsRetryBackoff
	}

	batchConcurrency := opts.BatchConcurrency
	if batchConcurrency <= 0 {
		batchConcurrency = defaultKnowsBatchLimit
	}

	cacheTTL := opts.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = defaultKnowsCacheTTL
	}

	cacheEntries := opts.CacheMaxEntries
	if cacheEntries <= 0 {
		cacheEntries = defaultKnowsCacheEntries
	}

	client := &knowsClient{
		baseURL: strings.TrimRight(apiBaseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		maxRetries:   maxRetries,
		retryBackoff: retryBackoff,
		cache:        newKnowsDetailCache(cacheTTL, cacheEntries),
	}

	factory := knowsToolFactory{
		client:           client,
		defaultDataScope: defaultScope,
		batchConcurrency: batchConcurrency,
	}

	return []Tool{
		factory.aiSearchTool(),
		factory.answerTool(),
		factory.batchAnswerTool(),
		factory.evidenceSummaryTool(),
		factory.evidenceHighlightTool(),
		factory.getPaperENTool(),
		factory.getPaperCNTool(),
		factory.getGuideTool(),
		factory.getMeetingTool(),
		factory.autoTaggingTool(),
		factory.listQuestionTool(),
		factory.listInterpretationTool(),
		factory.batchGetEvidenceDetailsTool(),
	}, nil
}

func newKnowsDetailCache(ttl time.Duration, maxEntries int) *knowsDetailCache {
	return &knowsDetailCache{
		entries:    make(map[string]knowsCacheEntry),
		order:      make([]string, 0, maxEntries),
		ttl:        ttl,
		maxEntries: maxEntries,
	}
}

func (c *knowsDetailCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		c.deleteLocked(key)
		return nil, false
	}

	return entry.value, true
}

func (c *knowsDetailCache) Set(key string, value interface{}) {
	if c.maxEntries <= 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[key]; !exists {
		for len(c.entries) >= c.maxEntries && len(c.order) > 0 {
			oldest := c.order[0]
			c.order = c.order[1:]
			delete(c.entries, oldest)
		}
		c.order = append(c.order, key)
	}

	c.entries[key] = knowsCacheEntry{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *knowsDetailCache) deleteLocked(key string) {
	delete(c.entries, key)
	for i, existing := range c.order {
		if existing == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

func (t *knowsTool) Name() string {
	return t.name
}

func (t *knowsTool) Description() string {
	return t.description
}

func (t *knowsTool) Parameters() map[string]interface{} {
	return t.parameters
}

func (t *knowsTool) Execute(ctx context.Context, args map[string]interface{}) *ToolResult {
	result, err := t.handler(ctx, args)
	if err != nil {
		return ErrorResult(err.Error()).WithError(err)
	}

	payload, err := json.Marshal(result)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to serialize knows response: %v", err)).WithError(err)
	}

	return NewToolResult(string(payload))
}

type knowsToolFactory struct {
	client           *knowsClient
	defaultDataScope []string
	batchConcurrency int
}

func (f knowsToolFactory) aiSearchTool() Tool {
	return &knowsTool{
		name:        "knows_ai_search",
		description: "Search clinical evidence and return a question_id plus evidence list. This should be used before answer generation.",
		parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"question": map[string]interface{}{
					"type":        "string",
					"description": "Question text to search evidence for.",
				},
				"data_scope": map[string]interface{}{
					"type":        "array",
					"description": "Optional evidence types. Allowed: PAPER, PAPER_CN, GUIDE, MEETING.",
					"items": map[string]interface{}{
						"type": "string",
						"enum": knowsAllDataScopes,
					},
				},
			},
			"required": []string{"question"},
		},
		handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			question, err := getRequiredString(args, "question")
			if err != nil {
				return nil, err
			}

			scope, err := getOptionalStringArray(args, "data_scope")
			if err != nil {
				return nil, err
			}

			if len(scope) == 0 {
				scope = append([]string(nil), f.defaultDataScope...)
			}

			normalizedScope, err := normalizeDataScopes(scope)
			if err != nil {
				return nil, err
			}

			return f.client.aiSearch(ctx, question, normalizedScope)
		},
	}
}

func (f knowsToolFactory) answerTool() Tool {
	return &knowsTool{
		name:        "knows_answer",
		description: "Generate one scenario-based answer from a question_id returned by knows_ai_search.",
		parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"question_id": map[string]interface{}{
					"type":        "string",
					"description": "question_id returned from knows_ai_search.",
				},
				"answer_type": map[string]interface{}{
					"type":        "string",
					"description": "Answer style.",
					"enum":        []string{"CLINICAL", "RESEARCH", "POPULAR_SCIENCE"},
				},
			},
			"required": []string{"question_id", "answer_type"},
		},
		handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			questionID, err := getRequiredString(args, "question_id")
			if err != nil {
				return nil, err
			}

			answerType, err := getRequiredString(args, "answer_type")
			if err != nil {
				return nil, err
			}

			answerType, err = normalizeAnswerType(answerType)
			if err != nil {
				return nil, err
			}

			return f.client.answer(ctx, questionID, answerType)
		},
	}
}

func (f knowsToolFactory) batchAnswerTool() Tool {
	return &knowsTool{
		name:        "knows_batch_answer",
		description: "Batch generate answers for multiple question_id values concurrently.",
		parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"requests": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"question_id": map[string]interface{}{
								"type": "string",
							},
							"answer_type": map[string]interface{}{
								"type": "string",
								"enum": []string{"CLINICAL", "RESEARCH", "POPULAR_SCIENCE"},
							},
						},
						"required": []string{"question_id", "answer_type"},
					},
				},
			},
			"required": []string{"requests"},
		},
		handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			requests, err := parseBatchAnswerRequests(args)
			if err != nil {
				return nil, err
			}

			type batchResult struct {
				index int
				data  map[string]interface{}
			}

			sem := make(chan struct{}, f.batchConcurrency)
			results := make([]map[string]interface{}, len(requests))
			ch := make(chan batchResult, len(requests))
			var wg sync.WaitGroup

			for i, req := range requests {
				i, req := i, req
				wg.Add(1)
				go func() {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					item := map[string]interface{}{
						"question_id": req.QuestionID,
					}

					data, err := f.client.answer(ctx, req.QuestionID, req.AnswerType)
					if err != nil {
						item["status"] = "error"
						item["error"] = err.Error()
					} else {
						item["status"] = "success"
						item["data"] = data
					}
					ch <- batchResult{index: i, data: item}
				}()
			}

			wg.Wait()
			close(ch)

			for item := range ch {
				results[item.index] = item.data
			}

			return results, nil
		},
	}
}

func (f knowsToolFactory) evidenceSummaryTool() Tool {
	return &knowsTool{
		name:        "knows_evidence_summary",
		description: "Get AI-generated summary for one evidence item.",
		parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"evidence_id": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"evidence_id"},
		},
		handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			evidenceID, err := getRequiredString(args, "evidence_id")
			if err != nil {
				return nil, err
			}
			return f.client.evidenceSummary(ctx, evidenceID)
		},
	}
}

func (f knowsToolFactory) evidenceHighlightTool() Tool {
	return &knowsTool{
		name:        "knows_evidence_highlight",
		description: "Get highlighted original evidence snippets for citation and traceability.",
		parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"evidence_id": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"evidence_id"},
		},
		handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			evidenceID, err := getRequiredString(args, "evidence_id")
			if err != nil {
				return nil, err
			}
			return f.client.evidenceHighlight(ctx, evidenceID)
		},
	}
}

func (f knowsToolFactory) getPaperENTool() Tool {
	return &knowsTool{
		name:        "knows_get_paper_en",
		description: "Get structured details of an English paper.",
		parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"evidence_id": map[string]interface{}{
					"type": "string",
				},
				"translate_to_chinese": map[string]interface{}{
					"type":        "boolean",
					"description": "Optional translation flag for title/abstract.",
				},
			},
			"required": []string{"evidence_id"},
		},
		handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			evidenceID, err := getRequiredString(args, "evidence_id")
			if err != nil {
				return nil, err
			}
			translate, err := getOptionalBoolPointer(args, "translate_to_chinese")
			if err != nil {
				return nil, err
			}
			return f.client.getPaperEN(ctx, evidenceID, translate)
		},
	}
}

func (f knowsToolFactory) getPaperCNTool() Tool {
	return &knowsTool{
		name:        "knows_get_paper_cn",
		description: "Get structured details of a Chinese paper.",
		parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"evidence_id": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"evidence_id"},
		},
		handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			evidenceID, err := getRequiredString(args, "evidence_id")
			if err != nil {
				return nil, err
			}
			return f.client.getPaperCN(ctx, evidenceID)
		},
	}
}

func (f knowsToolFactory) getGuideTool() Tool {
	return &knowsTool{
		name:        "knows_get_guide",
		description: "Get detailed content of a clinical guideline.",
		parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"evidence_id": map[string]interface{}{
					"type": "string",
				},
				"translate_to_chinese": map[string]interface{}{
					"type": "boolean",
				},
			},
			"required": []string{"evidence_id"},
		},
		handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			evidenceID, err := getRequiredString(args, "evidence_id")
			if err != nil {
				return nil, err
			}
			translate, err := getOptionalBoolPointer(args, "translate_to_chinese")
			if err != nil {
				return nil, err
			}
			return f.client.getGuide(ctx, evidenceID, translate)
		},
	}
}

func (f knowsToolFactory) getMeetingTool() Tool {
	return &knowsTool{
		name:        "knows_get_meeting",
		description: "Get detailed content of a medical meeting abstract.",
		parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"evidence_id": map[string]interface{}{
					"type": "string",
				},
				"translate_to_chinese": map[string]interface{}{
					"type": "boolean",
				},
			},
			"required": []string{"evidence_id"},
		},
		handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			evidenceID, err := getRequiredString(args, "evidence_id")
			if err != nil {
				return nil, err
			}
			translate, err := getOptionalBoolPointer(args, "translate_to_chinese")
			if err != nil {
				return nil, err
			}
			return f.client.getMeeting(ctx, evidenceID, translate)
		},
	}
}

func (f knowsToolFactory) autoTaggingTool() Tool {
	return &knowsTool{
		name:        "knows_auto_tagging",
		description: "Automatically extract tags and structured elements from text or evidence.",
		parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{
					"type": "string",
				},
				"evidence_id": map[string]interface{}{
					"type": "string",
				},
				"tagging_type": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"tagging_type"},
		},
		handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			taggingType, err := getRequiredString(args, "tagging_type")
			if err != nil {
				return nil, err
			}

			content, err := getOptionalString(args, "content")
			if err != nil {
				return nil, err
			}
			evidenceID, err := getOptionalString(args, "evidence_id")
			if err != nil {
				return nil, err
			}

			return f.client.autoTagging(ctx, content, evidenceID, taggingType)
		},
	}
}

func (f knowsToolFactory) listQuestionTool() Tool {
	return &knowsTool{
		name:        "knows_list_question",
		description: "List historical question records.",
		parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"from_time": map[string]interface{}{
					"type": "integer",
				},
				"to_time": map[string]interface{}{
					"type": "integer",
				},
				"page": map[string]interface{}{
					"type": "integer",
				},
				"page_size": map[string]interface{}{
					"type": "integer",
				},
			},
		},
		handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			fromTime, err := getOptionalInt64(args, "from_time")
			if err != nil {
				return nil, err
			}
			toTime, err := getOptionalInt64(args, "to_time")
			if err != nil {
				return nil, err
			}
			page, err := getOptionalInt64(args, "page")
			if err != nil {
				return nil, err
			}
			pageSize, err := getOptionalInt64(args, "page_size")
			if err != nil {
				return nil, err
			}

			return f.client.listQuestion(ctx, fromTime, toTime, page, pageSize)
		},
	}
}

func (f knowsToolFactory) listInterpretationTool() Tool {
	return &knowsTool{
		name:        "knows_list_interpretation",
		description: "List historical interpretation records.",
		parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"from_time": map[string]interface{}{
					"type": "integer",
				},
				"to_time": map[string]interface{}{
					"type": "integer",
				},
				"page": map[string]interface{}{
					"type": "integer",
				},
				"page_size": map[string]interface{}{
					"type": "integer",
				},
			},
		},
		handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			fromTime, err := getOptionalInt64(args, "from_time")
			if err != nil {
				return nil, err
			}
			toTime, err := getOptionalInt64(args, "to_time")
			if err != nil {
				return nil, err
			}
			page, err := getOptionalInt64(args, "page")
			if err != nil {
				return nil, err
			}
			pageSize, err := getOptionalInt64(args, "page_size")
			if err != nil {
				return nil, err
			}

			return f.client.listInterpretation(ctx, fromTime, toTime, page, pageSize)
		},
	}
}

func (f knowsToolFactory) batchGetEvidenceDetailsTool() Tool {
	return &knowsTool{
		name:        "knows_batch_get_evidence_details",
		description: "Batch get evidence details for PAPER, PAPER_CN, GUIDE, or MEETING.",
		parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"evidences": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"evidence_id": map[string]interface{}{
								"type": "string",
							},
							"type": map[string]interface{}{
								"type": "string",
								"enum": knowsAllDataScopes,
							},
						},
						"required": []string{"evidence_id", "type"},
					},
				},
				"translate_to_chinese": map[string]interface{}{
					"type": "boolean",
				},
			},
			"required": []string{"evidences"},
		},
		handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			evidences, err := parseBatchEvidenceRequests(args)
			if err != nil {
				return nil, err
			}
			translate, err := getOptionalBoolPointer(args, "translate_to_chinese")
			if err != nil {
				return nil, err
			}

			type batchResult struct {
				index int
				data  map[string]interface{}
			}

			sem := make(chan struct{}, f.batchConcurrency)
			results := make([]map[string]interface{}, len(evidences))
			ch := make(chan batchResult, len(evidences))
			var wg sync.WaitGroup

			for i, item := range evidences {
				i, item := i, item
				wg.Add(1)
				go func() {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					row := map[string]interface{}{
						"evidence_id": item.EvidenceID,
						"type":        item.Type,
					}

					data, err := f.client.fetchEvidenceDetail(ctx, item.EvidenceID, item.Type, translate)
					if err != nil {
						row["status"] = "error"
						row["error"] = err.Error()
					} else {
						row["status"] = "success"
						row["data"] = data
					}

					ch <- batchResult{index: i, data: row}
				}()
			}

			wg.Wait()
			close(ch)

			for item := range ch {
				results[item.index] = item.data
			}

			return results, nil
		},
	}
}

func (c *knowsClient) aiSearch(ctx context.Context, question string, dataScope []string) (interface{}, error) {
	return c.postJSON(ctx, "/knows/ai_search", map[string]interface{}{
		"query":      question,
		"data_scope": dataScope,
	})
}

func (c *knowsClient) answer(ctx context.Context, questionID, answerType string) (interface{}, error) {
	return c.postJSON(ctx, "/knows/answer", map[string]interface{}{
		"question_id": questionID,
		"answer_type": answerType,
	})
}

func (c *knowsClient) evidenceSummary(ctx context.Context, evidenceID string) (interface{}, error) {
	return c.postJSON(ctx, "/knows/evidence/summary", map[string]interface{}{
		"evidence_id": evidenceID,
	})
}

func (c *knowsClient) evidenceHighlight(ctx context.Context, evidenceID string) (interface{}, error) {
	return c.postJSON(ctx, "/knows/evidence/highlight", map[string]interface{}{
		"evidence_id": evidenceID,
	})
}

func (c *knowsClient) getPaperEN(ctx context.Context, evidenceID string, translate *bool) (interface{}, error) {
	translateKey := false
	if translate != nil {
		translateKey = *translate
	}
	cacheKey := fmt.Sprintf("PAPER:%s:%t", evidenceID, translateKey)
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached, nil
	}

	payload := map[string]interface{}{
		"evidence_id": evidenceID,
	}
	if translate != nil {
		payload["translate_to_chinese"] = *translate
	}

	data, err := c.postJSON(ctx, "/knows/evidence/get_paper_en", payload)
	if err != nil {
		return nil, err
	}
	c.cache.Set(cacheKey, data)
	return data, nil
}

func (c *knowsClient) getPaperCN(ctx context.Context, evidenceID string) (interface{}, error) {
	cacheKey := fmt.Sprintf("PAPER_CN:%s:false", evidenceID)
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached, nil
	}

	data, err := c.postJSON(ctx, "/knows/evidence/get_paper_cn", map[string]interface{}{
		"evidence_id": evidenceID,
	})
	if err != nil {
		return nil, err
	}
	c.cache.Set(cacheKey, data)
	return data, nil
}

func (c *knowsClient) getGuide(ctx context.Context, evidenceID string, translate *bool) (interface{}, error) {
	translateKey := false
	if translate != nil {
		translateKey = *translate
	}
	cacheKey := fmt.Sprintf("GUIDE:%s:%t", evidenceID, translateKey)
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached, nil
	}

	payload := map[string]interface{}{
		"evidence_id": evidenceID,
	}
	if translate != nil {
		payload["translate_to_chinese"] = *translate
	}

	data, err := c.postJSON(ctx, "/knows/evidence/get_guide", payload)
	if err != nil {
		return nil, err
	}
	c.cache.Set(cacheKey, data)
	return data, nil
}

func (c *knowsClient) getMeeting(ctx context.Context, evidenceID string, translate *bool) (interface{}, error) {
	translateKey := false
	if translate != nil {
		translateKey = *translate
	}
	cacheKey := fmt.Sprintf("MEETING:%s:%t", evidenceID, translateKey)
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached, nil
	}

	payload := map[string]interface{}{
		"evidence_id": evidenceID,
	}
	if translate != nil {
		payload["translate_to_chinese"] = *translate
	}

	data, err := c.postJSON(ctx, "/knows/evidence/get_meeting", payload)
	if err != nil {
		return nil, err
	}
	c.cache.Set(cacheKey, data)
	return data, nil
}

func (c *knowsClient) autoTagging(ctx context.Context, content, evidenceID, taggingType string) (interface{}, error) {
	payload := map[string]interface{}{
		"tagging_type": taggingType,
	}
	if content != "" {
		payload["content"] = content
	}
	if evidenceID != "" {
		payload["evidence_id"] = evidenceID
	}
	return c.postJSON(ctx, "/knows/auto_tagging", payload)
}

func (c *knowsClient) listQuestion(ctx context.Context, fromTime, toTime, page, pageSize *int64) (interface{}, error) {
	payload := map[string]interface{}{}
	if fromTime != nil {
		payload["from_time"] = *fromTime
	}
	if toTime != nil {
		payload["to_time"] = *toTime
	}
	if page != nil {
		payload["page"] = *page
	}
	if pageSize != nil {
		payload["page_size"] = *pageSize
	}
	return c.postJSON(ctx, "/knows/list_question", payload)
}

func (c *knowsClient) listInterpretation(ctx context.Context, fromTime, toTime, page, pageSize *int64) (interface{}, error) {
	payload := map[string]interface{}{}
	if fromTime != nil {
		payload["from_time"] = *fromTime
	}
	if toTime != nil {
		payload["to_time"] = *toTime
	}
	if page != nil {
		payload["page"] = *page
	}
	if pageSize != nil {
		payload["page_size"] = *pageSize
	}
	// Keep endpoint spelling for compatibility with KnowS API.
	return c.postJSON(ctx, "/knows/list_interpretion", payload)
}

func (c *knowsClient) fetchEvidenceDetail(ctx context.Context, evidenceID, evidenceType string, translate *bool) (interface{}, error) {
	switch evidenceType {
	case "PAPER":
		return c.getPaperEN(ctx, evidenceID, translate)
	case "PAPER_CN":
		return c.getPaperCN(ctx, evidenceID)
	case "GUIDE":
		return c.getGuide(ctx, evidenceID, translate)
	case "MEETING":
		return c.getMeeting(ctx, evidenceID, translate)
	default:
		return nil, fmt.Errorf("unsupported evidence type %q; allowed: PAPER, PAPER_CN, GUIDE, MEETING", evidenceType)
	}
}

func (c *knowsClient) postJSON(ctx context.Context, path string, payload interface{}) (interface{}, error) {
	if payload == nil {
		payload = map[string]interface{}{}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request for %s: %w", path, err)
	}

	url := c.baseURL + path
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed to create request for %s: %w", path, err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request to %s failed: %w", path, err)
			if c.shouldRetry(attempt, 0, err) {
				if waitErr := waitRetry(ctx, c.retryBackoff, attempt); waitErr != nil {
					return nil, waitErr
				}
				continue
			}
			return nil, lastErr
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("failed to read response from %s: %w", path, readErr)
			if c.shouldRetry(attempt, resp.StatusCode, readErr) {
				if waitErr := waitRetry(ctx, c.retryBackoff, attempt); waitErr != nil {
					return nil, waitErr
				}
				continue
			}
			return nil, lastErr
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("request to %s failed with status %d: %s", path, resp.StatusCode, truncateForError(string(body), 500))
			if c.shouldRetry(attempt, resp.StatusCode, nil) {
				if waitErr := waitRetry(ctx, c.retryBackoff, attempt); waitErr != nil {
					return nil, waitErr
				}
				continue
			}
			return nil, lastErr
		}

		if len(bytes.TrimSpace(body)) == 0 {
			return map[string]interface{}{}, nil
		}

		var raw interface{}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, fmt.Errorf("failed to decode response from %s: %w", path, err)
		}

		if obj, ok := raw.(map[string]interface{}); ok {
			if nested, ok := obj["data"]; ok && nested != nil {
				return nested, nil
			}
		}

		return raw, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("request to %s failed", path)
}

func (c *knowsClient) shouldRetry(attempt, statusCode int, err error) bool {
	if attempt >= c.maxRetries {
		return false
	}
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		var netErr net.Error
		if errors.As(err, &netErr) {
			return true
		}
		return true
	}
	return statusCode >= 500 && statusCode <= 599
}

func waitRetry(ctx context.Context, baseDelay time.Duration, attempt int) error {
	delay := baseDelay * time.Duration(1<<attempt)
	if delay > 8*time.Second {
		delay = 8 * time.Second
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func parseBatchAnswerRequests(args map[string]interface{}) ([]knowsBatchAnswerRequest, error) {
	raw, err := getRequiredArray(args, "requests")
	if err != nil {
		return nil, err
	}

	out := make([]knowsBatchAnswerRequest, 0, len(raw))
	for i, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("requests[%d] must be an object", i)
		}

		questionID, err := getRequiredString(m, "question_id")
		if err != nil {
			return nil, fmt.Errorf("requests[%d]: %w", i, err)
		}
		answerType, err := getRequiredString(m, "answer_type")
		if err != nil {
			return nil, fmt.Errorf("requests[%d]: %w", i, err)
		}
		answerType, err = normalizeAnswerType(answerType)
		if err != nil {
			return nil, fmt.Errorf("requests[%d]: %w", i, err)
		}

		out = append(out, knowsBatchAnswerRequest{
			QuestionID: questionID,
			AnswerType: answerType,
		})
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("requests must not be empty")
	}
	return out, nil
}

func parseBatchEvidenceRequests(args map[string]interface{}) ([]knowsBatchEvidenceRequest, error) {
	raw, err := getRequiredArray(args, "evidences")
	if err != nil {
		return nil, err
	}

	out := make([]knowsBatchEvidenceRequest, 0, len(raw))
	for i, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("evidences[%d] must be an object", i)
		}

		evidenceID, err := getRequiredString(m, "evidence_id")
		if err != nil {
			return nil, fmt.Errorf("evidences[%d]: %w", i, err)
		}
		evidenceType, err := getRequiredString(m, "type")
		if err != nil {
			return nil, fmt.Errorf("evidences[%d]: %w", i, err)
		}
		evidenceType, err = normalizeDataScope(evidenceType)
		if err != nil {
			return nil, fmt.Errorf("evidences[%d]: %w", i, err)
		}

		out = append(out, knowsBatchEvidenceRequest{
			EvidenceID: evidenceID,
			Type:       evidenceType,
		})
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("evidences must not be empty")
	}
	return out, nil
}

func getRequiredString(args map[string]interface{}, key string) (string, error) {
	raw, ok := args[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	str, ok := raw.(string)
	if !ok || strings.TrimSpace(str) == "" {
		return "", fmt.Errorf("%s must be a non-empty string", key)
	}
	return strings.TrimSpace(str), nil
}

func getOptionalString(args map[string]interface{}, key string) (string, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return "", nil
	}
	str, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return strings.TrimSpace(str), nil
}

func getOptionalBoolPointer(args map[string]interface{}, key string) (*bool, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil, nil
	}

	switch v := raw.(type) {
	case bool:
		value := v
		return &value, nil
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return nil, fmt.Errorf("%s must be a boolean", key)
		}
		return &parsed, nil
	default:
		return nil, fmt.Errorf("%s must be a boolean", key)
	}
}

func getOptionalInt64(args map[string]interface{}, key string) (*int64, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil, nil
	}

	switch v := raw.(type) {
	case float64:
		value := int64(v)
		return &value, nil
	case int:
		value := int64(v)
		return &value, nil
	case int64:
		value := v
		return &value, nil
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return nil, fmt.Errorf("%s must be an integer", key)
		}
		return &n, nil
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s must be an integer", key)
		}
		return &n, nil
	default:
		return nil, fmt.Errorf("%s must be an integer", key)
	}
}

func getRequiredArray(args map[string]interface{}, key string) ([]interface{}, error) {
	raw, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}
	return arr, nil
}

func getOptionalStringArray(args map[string]interface{}, key string) ([]string, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil, nil
	}

	switch v := raw.(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			text := strings.TrimSpace(item)
			if text == "" {
				continue
			}
			out = append(out, text)
		}
		return out, nil
	case []interface{}:
		out := make([]string, 0, len(v))
		for i, item := range v {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%s[%d] must be a string", key, i)
			}
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			out = append(out, text)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%s must be an array of strings", key)
	}
}

func normalizeDataScopes(scopes []string) ([]string, error) {
	if len(scopes) == 0 {
		return nil, nil
	}

	out := make([]string, 0, len(scopes))
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		normalized, err := normalizeDataScope(scope)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out, nil
}

func normalizeDataScope(scope string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(scope))
	if normalized == "" {
		return "", fmt.Errorf("data scope must be non-empty")
	}
	if _, ok := knowsAllowedDataScopes[normalized]; !ok {
		return "", fmt.Errorf("unsupported data scope %q; allowed: PAPER, PAPER_CN, GUIDE, MEETING", scope)
	}
	return normalized, nil
}

func normalizeAnswerType(answerType string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(answerType))
	if normalized == "" {
		return "", fmt.Errorf("answer_type must be non-empty")
	}
	if _, ok := knowsAllowedAnswerTypes[normalized]; !ok {
		return "", fmt.Errorf("unsupported answer_type %q; allowed: CLINICAL, RESEARCH, POPULAR_SCIENCE", answerType)
	}
	return normalized, nil
}

func truncateForError(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max] + "..."
}
