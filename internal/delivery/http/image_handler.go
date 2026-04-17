// Package http — Image handler'ı.
// Gin handler ile net/http handler arasındaki farkı görmek için ideal dosya.

package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ryuka/art-dataset-collector/internal/usecase"
)

// ImageHandler, Image ile ilgili HTTP isteklerini karşılar.
// Sadece ImageUseCase'e bağımlıdır — veri tabanını bilmez.
type ImageHandler struct {
	imageUC *usecase.ImageUseCase
}

// NewImageHandler, yeni bir ImageHandler örneği oluşturur.
func NewImageHandler(imageUC *usecase.ImageUseCase) *ImageHandler {
	return &ImageHandler{imageUC: imageUC}
}

// GetNext, kullanıcıya gösterilecek sonraki görseli döner.
//
// Route: GET /api/images/next
// Header: X-API-Key: <api_key>
//
// Başarılı Yanıt (200):
//
//	{
//	  "image_id":      "550e8400-e29b-41d4-a716-446655440000",
//	  "thumbnail_url": "https://drive.google.com/thumbnail?id=ABC&sz=w1000",
//	  "file_name":     "monet_haystacks.jpg",
//	  "rating_count":  42
//	}
//
// ── net/http vs Gin karşılaştırması ──────────────────────────
//
//	net/http handler imzası:
//	  func GetNext(w http.ResponseWriter, r *http.Request)
//
//	Gin handler imzası:
//	  func (h *ImageHandler) GetNext(c *gin.Context)
//
//	c *gin.Context, w + r'yi tek bir struct'ta birleştirir:
//	  c.Request  = r (http.Request — aynen aynı!)
//	  c.Writer   = w (ResponseWriter arayüzü)
//	  c.JSON()   = json.Encode(w) + w.WriteHeader() kısaltması
func (h *ImageHandler) GetNext(c *gin.Context) {
	// c.Request.Context(): İstek iptal edilirse (kullanıcı bağlantıyı kesti)
	// veritabanı sorgusunu da iptal etmek için context geçiriyoruz.
	// net/http karşılığı: r.Context()
	ctx := c.Request.Context()

	// Use Case'i çağır
	response, err := h.imageUC.GetNextImage(ctx)
	if err != nil {
		// Hata varsa merkezi hata işleyiciye yönlendir
		respondWithError(c, err)
		return
	}

	// Başarılı yanıt: JSON olarak gönder
	// net/http karşılığı:
	//   w.Header().Set("Content-Type", "application/json")
	//   w.WriteHeader(http.StatusOK)
	//   json.NewEncoder(w).Encode(response)
	c.JSON(http.StatusOK, response)
}

// GetStats, sistem istatistiklerini döner.
//
// Route: GET /api/stats
// Header: X-API-Key: <api_key>
//
// Başarılı Yanıt (200):
//
//	{
//	  "total_images":   14000,
//	  "rated_images":   8500,
//	  "unrated_images": 5500,
//	  "total_ratings":  23400,
//	  "completion_pct": 60.71
//	}
func (h *ImageHandler) GetStats(c *gin.Context) {
	ctx := c.Request.Context()

	response, err := h.imageUC.GetStats(ctx)
	if err != nil {
		respondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}
