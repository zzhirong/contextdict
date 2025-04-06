package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/zzhirong/contextdict/config"
	"github.com/zzhirong/contextdict/internal/handlers"
	"github.com/zzhirong/contextdict/internal/metrics"
	"github.com/zzhirong/contextdict/internal/models"
)

// --- Mocks ---

// MockRepository is a mock type for database.Repository
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) FindTranslation(ctx context.Context, keyword, contextStr string) (*models.TranslationResponse, error) {
	args := m.Called(ctx, keyword, contextStr)
	// Need type assertion for the first return value
	res, _ := args.Get(0).(*models.TranslationResponse)
	return res, args.Error(1)
}

func (m *MockRepository) CreateTranslation(ctx context.Context, record *models.TranslationResponse) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *MockRepository) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockAIClient is a mock type for ai.Client
type MockAIClient struct {
	mock.Mock
}

func (m *MockAIClient) Generate(ctx context.Context, prompt string, texts ...string) (string, error) {
	// To make matching more flexible, especially with varying texts,
	// we might need to use mock.Anything or more specific argument matchers.
	// For simplicity now, match exact arguments.
	args := m.Called(ctx, prompt, texts)
	return args.String(0), args.Error(1)
}

// Helper to create a test Gin context and recorder
func setupTestRouter(h *handlers.APIHandler) (*gin.Engine, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	router := gin.New() // Use New, not Default, for test isolation

	// Register routes like in server.go but simplified for specific handler tests
	router.GET("/api/translate", h.Translate)
	router.GET("/api/format", h.Format)
	router.GET("/api/summarize", h.Summarize)

	return router, w
}

// --- Test Cases ---

// 测试辅助函数
type testSetup struct {
	repo     *MockRepository
	ai       *MockAIClient
	metrics  *metrics.Metrics
	registry *prometheus.Registry
	cfg      *config.Config
}

func newTestSetup() *testSetup {
	registry := prometheus.NewRegistry()
	return &testSetup{
		repo: new(MockRepository),
		ai:   new(MockAIClient),
		metrics: &metrics.Metrics{
			TranslationCounter: promauto.With(registry).NewCounterVec(
				prometheus.CounterOpts{Name: "test_reqs", Help: "test"},
				[]string{"type"},
			),
			TranslationCacheHitCounter: promauto.With(registry).NewCounterVec(
				prometheus.CounterOpts{Name: "test_cache_hits", Help: "test"},
				[]string{"type"},
			),
		},
		registry: registry,
		cfg:      &config.Config{},
	}
}

func (ts *testSetup) newHandler() (*handlers.APIHandler, *gin.Engine, *httptest.ResponseRecorder) {
	handler := handlers.NewAPIHandler(ts.repo, ts.ai, ts.metrics, &ts.cfg.Prompts)
	router, w := setupTestRouter(handler)
	return handler, router, w
}

// 验证 Prometheus 指标
func (ts *testSetup) assertMetric(t *testing.T, name, label string, expected float64) {
	var metric *prometheus.CounterVec
	switch name {
	case "requests":
		metric = ts.metrics.TranslationCounter
	case "cache_hits":
		metric = ts.metrics.TranslationCacheHitCounter
	default:
		t.Fatalf("Unknown metric name: %s", name)
	}

	actual := testutil.ToFloat64(metric.WithLabelValues(label))
	assert.Equal(t, expected, actual, "Metric %s{type=%s} should be %v", name, label, expected)
}

// 测试用例
func TestAPIHandler_MissingKeyword(t *testing.T) {
	ts := newTestSetup()
	_, router, w := ts.newHandler()

	// 测试 Format 接口
	req, _ := http.NewRequest(http.MethodGet, "/api/format", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Missing required parameter: keyword")

	// 测试 Summarize 接口
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/summarize", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Missing required parameter: keyword")

	// 测试 Translate 接口
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/translate", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Missing required parameter: keyword")
}

func TestAPIHandler_Translate_CacheHit(t *testing.T) {
	ts := newTestSetup()
	keyword, context := "hello", "greeting"
	cachedResponse := &models.TranslationResponse{
		Keyword:     keyword,
		Context:     context,
		Translation: "你好",
	}

	ts.repo.On("FindTranslation", mock.Anything, keyword, context).Return(cachedResponse, nil)

	_, router, w := ts.newHandler()
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/translate?keyword=%s&context=%s",
		url.QueryEscape(keyword), url.QueryEscape(context)), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"result":"你好"}`, w.Body.String())

	ts.repo.AssertExpectations(t)
	ts.ai.AssertNotCalled(t, "Generate")
	ts.assertMetric(t, "cache_hits", "translate", 1)
}

func TestAPIHandler_Translate_CacheMiss_AI_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	mockAI := new(MockAIClient)
	mockMetricsRegistry := prometheus.NewRegistry() // Use a specific registry for tests
	mockMetrics := &metrics.Metrics{
		TranslationCounter:         promauto.With(mockMetricsRegistry).NewCounterVec(prometheus.CounterOpts{Name: "test_reqs"}, []string{"type"}),
		TranslationCacheHitCounter: promauto.With(mockMetricsRegistry).NewCounterVec(prometheus.CounterOpts{Name: "test_cache_hits"}, []string{"type"}),
	}

	cfg := &config.Config{ // Config needed for prompts
		Prompts: config.PromptConfig{
			TranslateOnContext: "Translate '{keyword}' context '{context}'",
			TranslateOrFormat:  "Translate '{keyword}'", // For the no-context case
		},
	}

	// Input
	keyword := "world"
	contextStr := "place"
	aiTranslation := "世界"

	// Mock Interactions
	// 1. Cache miss
	mockRepo.On("FindTranslation", mock.Anything, keyword, contextStr).Return(nil, nil) // Return nil, nil for cache miss
	// 2. AI call (using context prompt)
	mockAI.On("Generate", mock.Anything, cfg.Prompts.TranslateOnContext, []string{keyword, contextStr}).Return(aiTranslation, nil)
	// 3. Cache creation
	// Use mock.MatchedBy to check the record content without knowing the ID/timestamps
	mockRepo.On("CreateTranslation", mock.Anything, mock.MatchedBy(func(resp *models.TranslationResponse) bool {
		return resp.Keyword == keyword && resp.Context == contextStr && resp.Translation == aiTranslation
	})).Return(nil)

	handler := handlers.NewAPIHandler(mockRepo, mockAI, mockMetrics, &cfg.Prompts)
	router, w := setupTestRouter(handler)

	req, _ := http.NewRequest(http.MethodGet, "/api/translate?keyword="+keyword+"&context="+contextStr, nil)
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	expectedBody := `{"result":"` + aiTranslation + `"}`
	assert.JSONEq(t, expectedBody, w.Body.String())

	mockRepo.AssertCalled(t, "FindTranslation", mock.Anything, keyword, contextStr)
	mockAI.AssertCalled(t, "Generate", mock.Anything, cfg.Prompts.TranslateOnContext, []string{keyword, contextStr})
	mockRepo.AssertCalled(t, "CreateTranslation", mock.Anything, mock.MatchedBy(func(resp *models.TranslationResponse) bool {
		return resp.Keyword == keyword && resp.Context == contextStr && resp.Translation == aiTranslation
	}))

	// Check counter metric (requires Prometheus test helpers or direct value check if possible)
	// pcReq := testutil.ToFloat64(mockMetrics.TranslationCounter.WithLabelValues("translate_context"))
	// assert.Equal(t, float64(1), pcReq)
	// pcHit := testutil.ToFloat64(mockMetrics.TranslationCacheHitCounter.WithLabelValues("translate"))
	// assert.Equal(t, float64(0), pcHit) // Cache miss
}

func TestAPIHandler_Translate_CacheMiss_AI_Fail(t *testing.T) {
	mockRepo := new(MockRepository)
	mockAI := new(MockAIClient)
	mockMetricsRegistry := prometheus.NewRegistry()
	mockMetrics := &metrics.Metrics{
		TranslationCounter:         promauto.With(mockMetricsRegistry).NewCounterVec(prometheus.CounterOpts{Name: "test_reqs"}, []string{"type"}),
		TranslationCacheHitCounter: promauto.With(mockMetricsRegistry).NewCounterVec(prometheus.CounterOpts{Name: "test_cache_hits"}, []string{"type"}),
	}

	cfg := &config.Config{
		Prompts: config.PromptConfig{TranslateOrFormat: "Translate '{keyword}'"},
	}
	keyword := "fail"

	// Mock Interactions
	// 1. Cache miss
	mockRepo.On("FindTranslation", mock.Anything, keyword, "").Return(nil, nil)
	// 2. AI call fails
	aiError := errors.New("AI service unreachable")
	mockAI.On("Generate", mock.Anything, cfg.Prompts.TranslateOrFormat, []string{keyword}).Return("", aiError)

	handler := handlers.NewAPIHandler(mockRepo, mockAI, mockMetrics, &cfg.Prompts)
	router, w := setupTestRouter(handler)

	req, _ := http.NewRequest(http.MethodGet, "/api/translate?keyword="+keyword, nil)
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "AI service failed") // Check for user-friendly error

	mockRepo.AssertCalled(t, "FindTranslation", mock.Anything, keyword, "")
	mockAI.AssertCalled(t, "Generate", mock.Anything, cfg.Prompts.TranslateOrFormat, []string{keyword})
	mockRepo.AssertNotCalled(t, "CreateTranslation", mock.Anything, mock.Anything) // Should not cache on failure

	// Check request counter incremented even on failure
	// pcReq := testutil.ToFloat64(mockMetrics.TranslationCounter.WithLabelValues("translate"))
	// assert.Equal(t, float64(1), pcReq)
}

func TestAPIHandler_Translate_BadRequest_Binding(t *testing.T) {
	// No mocks needed as binding happens before handlers
	handler := handlers.NewAPIHandler(nil, nil, nil, nil) // Pass nil dependencies
	router, w := setupTestRouter(handler)

	// Send request with invalid param type (e.g., keyword as array)
	req, _ := http.NewRequest(http.MethodGet, "/api/translate?keyword[]=a&keyword[]=b", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Missing required parameter: keyword")
}

func TestAPIHandler_Format_Success(t *testing.T) {
	mockRepo := new(MockRepository) // Repo not used by Format
	mockAI := new(MockAIClient)
	mockMetricsRegistry := prometheus.NewRegistry()
	mockMetrics := &metrics.Metrics{
		TranslationCounter: promauto.With(mockMetricsRegistry).NewCounterVec(prometheus.CounterOpts{Name: "test_reqs"}, []string{"type"}),
		// Other metrics not directly used by Format
	}

	cfg := &config.Config{
		Prompts: config.PromptConfig{Format: "Format '{keyword}'"},
	}
	keyword := "some code snippet"
	aiFormatted := "`some code snippet`"

	// Mock Interactions
	mockAI.On("Generate", mock.Anything, cfg.Prompts.Format, []string{keyword}).Return(aiFormatted, nil)

	handler := handlers.NewAPIHandler(mockRepo, mockAI, mockMetrics, &cfg.Prompts)
	router, w := setupTestRouter(handler)

	req, _ := http.NewRequest(http.MethodGet, "/api/format?keyword="+url.QueryEscape(keyword), nil)
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	expectedBody := `{"result":"` + aiFormatted + `"}`
	assert.JSONEq(t, expectedBody, w.Body.String())

	mockAI.AssertCalled(t, "Generate", mock.Anything, cfg.Prompts.Format, []string{keyword})

	// Check counter metric
	pcReq := testutil.ToFloat64(mockMetrics.TranslationCounter.WithLabelValues("format"))
	assert.Equal(t, float64(1), pcReq)
}

func TestAPIHandler_Format_AI_Fail(t *testing.T) {
	mockRepo := new(MockRepository)
	mockAI := new(MockAIClient)
	mockMetricsRegistry := prometheus.NewRegistry()
	mockMetrics := &metrics.Metrics{
		TranslationCounter: promauto.With(mockMetricsRegistry).NewCounterVec(prometheus.CounterOpts{Name: "test_reqs"}, []string{"type"}),
	}

	cfg := &config.Config{
		Prompts: config.PromptConfig{Format: "Format '{keyword}'"},
	}
	keyword := "bad format"
	aiError := errors.New("AI format error")

	// Mock Interactions
	mockAI.On("Generate", mock.Anything, cfg.Prompts.Format, []string{keyword}).Return("", aiError)

	handler := handlers.NewAPIHandler(mockRepo, mockAI, mockMetrics, &cfg.Prompts)
	router, w := setupTestRouter(handler)

	req, _ := http.NewRequest(http.MethodGet, "/api/format?keyword="+url.QueryEscape(keyword), nil)
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "AI service failed to format text")

	mockAI.AssertCalled(t, "Generate", mock.Anything, cfg.Prompts.Format, []string{keyword})
	// Check counter metric incremented
	// pcReq := testutil.ToFloat64(mockMetrics.TranslationCounter.WithLabelValues("format"))
	// assert.Equal(t, float64(1), pcReq)
}

func TestAPIHandler_Summarize_Success(t *testing.T) {
	mockRepo := new(MockRepository) // Repo not used
	mockAI := new(MockAIClient)
	mockMetricsRegistry := prometheus.NewRegistry()
	mockMetrics := &metrics.Metrics{
		TranslationCounter: promauto.With(mockMetricsRegistry).NewCounterVec(prometheus.CounterOpts{Name: "test_reqs"}, []string{"type"}),
	}

	cfg := &config.Config{
		Prompts: config.PromptConfig{Summarize: "Summarize '{keyword}'"},
	}
	keyword := "a long text to summarize"
	aiSummary := "short summary"

	// Mock Interactions
	mockAI.On("Generate", mock.Anything, cfg.Prompts.Summarize, []string{keyword}).Return(aiSummary, nil)

	handler := handlers.NewAPIHandler(mockRepo, mockAI, mockMetrics, &cfg.Prompts)
	router, w := setupTestRouter(handler)

	req, _ := http.NewRequest(http.MethodGet, "/api/summarize?keyword="+url.QueryEscape(keyword), nil)
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	expectedBody := `{"result":"` + aiSummary + `"}`
	assert.JSONEq(t, expectedBody, w.Body.String())

	mockAI.AssertCalled(t, "Generate", mock.Anything, cfg.Prompts.Summarize, []string{keyword})
	// Check counter metric
	// pcReq := testutil.ToFloat64(mockMetrics.TranslationCounter.WithLabelValues("summarize"))
	// assert.Equal(t, float64(1), pcReq)
}

func TestAPIHandler_Summarize_AI_Fail(t *testing.T) {
	mockRepo := new(MockRepository)
	mockAI := new(MockAIClient)
	mockMetricsRegistry := prometheus.NewRegistry()
	mockMetrics := &metrics.Metrics{
		TranslationCounter: promauto.With(mockMetricsRegistry).NewCounterVec(prometheus.CounterOpts{Name: "test_reqs"}, []string{"type"}),
	}
	cfg := &config.Config{
		Prompts: config.PromptConfig{Summarize: "Summarize '{keyword}'"},
	}
	keyword := "bad summary"
	aiError := errors.New("AI summarize error")

	// Mock Interactions
	mockAI.On("Generate", mock.Anything, cfg.Prompts.Summarize, []string{keyword}).Return("", aiError)

	handler := handlers.NewAPIHandler(mockRepo, mockAI, mockMetrics, &cfg.Prompts)
	router, w := setupTestRouter(handler)

	req, _ := http.NewRequest(http.MethodGet, "/api/summarize?keyword="+url.QueryEscape(keyword), nil)
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "AI service failed to summarize text")

	mockAI.AssertCalled(t, "Generate", mock.Anything, cfg.Prompts.Summarize, []string{keyword})
}
