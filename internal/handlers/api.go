package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zzhirong/contextdict/config"
	"github.com/zzhirong/contextdict/internal/ai"
	"github.com/zzhirong/contextdict/internal/database"
	"github.com/zzhirong/contextdict/internal/metrics"
	"github.com/zzhirong/contextdict/internal/models"
	"gorm.io/gorm"
)

type query struct {
	Keyword string `form:"keyword" binding:"required"`
	Context string `form:"context"`
}

type APIHandler struct {
	Repo     database.Repository
	AIClient ai.Client
	Metrics  *metrics.Metrics
	Prompts  *config.PromptConfig
}

func NewAPIHandler(repo database.Repository, aiClient ai.Client, metrics *metrics.Metrics, prompts *config.PromptConfig) *APIHandler {
	return &APIHandler{
		Repo:     repo,
		AIClient: aiClient,
		Metrics:  metrics,
		Prompts:  prompts,
	}
}

// 添加辅助函数
func checkKeyword(c *gin.Context) (*query, bool) {
	q := &query{}
	if err := c.BindQuery(q); err != nil {
		log.Printf("Missing required parameter: keyword")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required parameter: keyword"})
		return nil, false
	}
	return q, true
}

func (h *APIHandler) Translate(c *gin.Context) {
	q, ok := checkKeyword(c)
	if !ok {
		return
	}
	cachedResult, err := h.Repo.FindTranslation(c.Request.Context(), q.Keyword, q.Context)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("Error checking cache for keyword='%s', context='%s': %v", q.Keyword, q.Context, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error checking cache"})
		return
	}

	if cachedResult != nil {
		log.Printf("Cache hit for keyword='%s', context='%s'", q.Keyword, q.Context)
		h.Metrics.TranslationCacheHitCounter.WithLabelValues("translate").Inc()
		c.JSON(http.StatusOK, gin.H{"result": cachedResult.Translation})
		return
	}

	log.Printf("Cache miss for keyword='%s', context='%s'. Querying AI.", q.Keyword, q.Context)

	var translation string
	var aiErr error
	var promptTypeLabel string

	if q.Context != "" {
		promptTypeLabel = "translate_context"
		translation, aiErr = h.AIClient.Generate(c.Request.Context(), h.Prompts.TranslateOnContext, q.Keyword, q.Context)
	} else {
		promptTypeLabel = "translate"
		translation, aiErr = h.AIClient.Generate(c.Request.Context(), h.Prompts.TranslateOrFormat, q.Keyword)
	}

	h.Metrics.TranslationCounter.WithLabelValues(promptTypeLabel).Inc()

	if aiErr != nil {
		log.Printf("AI generation failed for keyword='%s', context='%s': %v", q.Keyword, q.Context, aiErr)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI service failed to generate translation"})
		return
	}

	if translation == "" {
		log.Printf("AI returned empty translation for keyword='%s',context='%s'",
			q.Keyword, q.Context)
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"error": "AI service returned an empty result"},
		)
		return
	}

	newRecord := &models.TranslationResponse{
		Keyword:     q.Keyword,
		Context:     q.Context,
		Translation: translation,
	}
	if err := h.Repo.CreateTranslation(c.Request.Context(), newRecord); err != nil {
		log.Printf("Error caching translation for keyword='%s', context='%s': %v", q.Keyword, q.Context, err)
	} else {
		log.Printf("Successfully cached translation for keyword='%s', context='%s'", q.Keyword, q.Context)
	}

	c.JSON(http.StatusOK, gin.H{"result": translation})
}

func (h *APIHandler) Format(c *gin.Context) {
	q, ok := checkKeyword(c)
	if !ok {
		return
	}

	h.Metrics.TranslationCounter.WithLabelValues("format").Inc()

	result, err := h.AIClient.Generate(c.Request.Context(), h.Prompts.Format, q.Keyword)
	if err != nil {
		log.Printf("AI generation failed for format keyword='%s': %v", q.Keyword, err)
		c.JSON(http.StatusServiceUnavailable,
			gin.H{"error": "AI service failed to format text"})
		return
	}

	if result == "" {
		log.Printf("AI returned empty format for keyword='%s'", q.Keyword)
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": "AI service returned an empty result"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": result})
}

func (h *APIHandler) Summarize(c *gin.Context) {
	q, ok := checkKeyword(c)
	if !ok {
		return
	}

	h.Metrics.TranslationCounter.WithLabelValues("summarize").Inc()

	result, err := h.AIClient.Generate(c.Request.Context(), h.Prompts.Summarize, q.Keyword)
	if err != nil {
		log.Printf("AI generation failed for summarize keyword='%s': %v", q.Keyword, err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI service failed to summarize text"})
		return
	}

	if result == "" {
		log.Printf("AI returned empty summary for keyword='%s'", q.Keyword)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI service returned an empty result"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": result})
}
