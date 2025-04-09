package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zzhirong/contextdict/internal/ai"
	"github.com/zzhirong/contextdict/internal/database"
	"github.com/zzhirong/contextdict/internal/metrics"
	"github.com/zzhirong/contextdict/internal/models"
	"gorm.io/gorm"
)

type query struct {
	Text     string `form:"text" binding:"required"`
	Role     string `form:"role" binding:"required"`
	Selected string `form:"selected"`
}

type APIHandler struct {
	Repo     database.Repository
	AIClient ai.Client
	Metrics  *metrics.Metrics
	Prompts  map[string]string
}

func NewAPIHandler(repo database.Repository, aiClient ai.Client, metrics *metrics.Metrics, prompts map[string]string) *APIHandler {
	return &APIHandler{
		Repo:     repo,
		AIClient: aiClient,
		Metrics:  metrics,
		Prompts:  prompts,
	}
}

// 添加辅助函数
func checkText(c *gin.Context) (*query, bool) {
	q := &query{}
	if err := c.BindQuery(q); err != nil {
		log.Printf("Missing required parameter: text")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required parameter: text"})
		return nil, false
	}
	return q, true
}

func (h *APIHandler) Translate(c *gin.Context) {
	log.Println("++++++++++++", c.Request.Header["Cf-Connecting-Ip"])
	q, ok := checkText(c)
	if !ok {
		return
	}
	cachedResult, err := h.Repo.FindTranslation(c.Request.Context(), q.Text, q.Selected)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("Error checking cache for text='%s', selected='%s': %v", q.Text, q.Selected, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error checking cache"})
		return
	}

	if cachedResult != nil {
		log.Printf("Cache hit for text='%s', context='%s'", q.Text, q.Selected)
		h.Metrics.TranslationCacheHitCounter.WithLabelValues("translate").Inc()
		c.JSON(http.StatusOK, gin.H{"result": cachedResult.Translation})
		return
	}

	log.Printf("Cache miss for text='%s', selected='%s'. Querying AI.", q.Text, q.Selected)

	var translation string
	var aiErr error
	var promptTypeLabel string

	if q.Selected != "" {
		promptTypeLabel = "translate_selected"
		translation, aiErr = h.AIClient.Generate(
			c.Request.Context(),
			h.Prompts["TranslateOnSelected"],
			q.Selected,
			q.Text,
		)
	} else {
		promptTypeLabel = "translate"
		translation, aiErr = h.AIClient.Generate(c.Request.Context(), h.Prompts["TranslateOrFormat"], q.Text)
	}

	h.Metrics.TranslationCounter.WithLabelValues(promptTypeLabel).Inc()

	if aiErr != nil {
		log.Printf("AI generation failed for text='%s', selected='%s': %v",
			q.Text, q.Selected, aiErr)
		c.JSON(http.StatusServiceUnavailable,
			gin.H{"error": "AI service failed to generate translation"})
		return
	}

	if translation == "" {
		log.Printf(
			"AI returned empty translation for text='%s',selected='%s'",
			q.Text,
			q.Selected,
		)
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"error": "AI service returned an empty result"},
		)
		return
	}

	newRecord := &models.TranslationResponse{
		Text:        q.Text,
		Selected:    q.Selected,
		Translation: translation,
	}
	if err := h.Repo.CreateTranslation(c.Request.Context(), newRecord); err != nil {
		log.Printf("Error caching translation for text='%s', selected='%s': %v", q.Text, q.Selected, err)
	} else {
		log.Printf("Successfully cached translation for text='%s', selected='%s'", q.Text, q.Selected)
	}

	c.JSON(http.StatusOK, gin.H{"result": translation})
}

func (h *APIHandler) Handle(c *gin.Context) {
	q, ok := checkText(c)
	if !ok {
		return
	}
	if q.Role == "translate" {
		h.Translate(c)
		return
	}
	h.Metrics.TranslationCounter.WithLabelValues(q.Role).Inc()

	var prompt string
	if prompt, ok = h.Prompts[q.Role]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role"})
		return
	}

	result, err := h.AIClient.Generate(c.Request.Context(), prompt, q.Text)
	if err != nil {
		log.Printf("AI generation failed for %s text='%s': %v", q.Role, q.Text, err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI service failed to process text"})
		return
	}

	if result == "" {
		log.Printf("AI returned empty result for %s text='%s'", q.Role, q.Text)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI service returned an empty result"})
		return
	}

	fmt.Printf("The response length: %d\n", len(result))
	c.JSON(http.StatusOK, gin.H{"result": result})
}
