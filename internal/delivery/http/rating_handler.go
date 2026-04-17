// Package http — Rating handler'ı.
// POST isteği ile gelen JSON body'nin nasıl işlendiğini gösterir.

package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ryuka/art-dataset-collector/internal/usecase"
)

// RatingHandler, puan işlemlerini yöneten HTTP handler'ıdır.
type RatingHandler struct {
	ratingUC *usecase.RatingUseCase
}

// NewRatingHandler, yeni bir RatingHandler örneği oluşturur.
func NewRatingHandler(ratingUC *usecase.RatingUseCase) *RatingHandler {
	return &RatingHandler{ratingUC: ratingUC}
}

// Create, yeni bir puan kaydı oluşturur.
//
// Route: POST /api/ratings
// Header: X-API-Key: <api_key>
// Content-Type: application/json
//
// İstek Gövdesi:
//
//	{
//	  "image_id":     "550e8400-e29b-41d4-a716-446655440000",
//	  "score":        7,
//	  "age":          25,
//	  "gender":       "female",
//	  "city":         "İstanbul",
//	  "knows_artist": false
//	}
//
// Başarılı Yanıt (201 Created):
//
//	{
//	  "id":         "660e8400-e29b-41d4-a716-446655440001",
//	  "image_id":   "550e8400-e29b-41d4-a716-446655440000",
//	  "score":      7,
//	  "created_at": "2024-01-15T14:30:00Z"
//	}
func (h *RatingHandler) Create(c *gin.Context) {
	// ── JSON Body'yi DTO'ya bağla ─────────────────────────────
	// c.ShouldBindJSON: Body'yi okur, JSON parse eder, struct'a atar,
	//   VE binding tag'lerini (required, min, max) doğrular.
	//
	// net/http karşılığı (manuel — çok daha uzun):
	//   var req usecase.CreateRatingRequest
	//   body, _ := io.ReadAll(r.Body)
	//   if err := json.Unmarshal(body, &req); err != nil { ... }
	//   // Sonra her alanı tek tek doğrula: if req.Score < 1 || req.Score > 10 { ... }
	//
	// Gin bunu tek satırda yapıyor:
	var req usecase.CreateRatingRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		// Binding hatası: JSON parse hatası veya required alan eksik
		// Örn: score=15 verildiyse "score must be max 10" hatası döner
		// net/http karşılığı: w.WriteHeader(400); json.Encode(w, errMsg)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "geçersiz istek formatı",
			"detail": err.Error(), // Hangi alan neden geçersiz?
		})
		return
	}

	// Use Case'i çağır
	ctx := c.Request.Context() // net/http karşılığı: r.Context()

	response, err := h.ratingUC.CreateRating(ctx, &req)
	if err != nil {
		respondWithError(c, err)
		return
	}

	// 201 Created: Yeni kaynak oluşturuldu
	// Not: Başarılı POST için 200 değil 201 kullanmak REST standardıdır
	// net/http karşılığı: w.WriteHeader(http.StatusCreated); json.Encode(w, response)
	c.JSON(http.StatusCreated, response)
}
