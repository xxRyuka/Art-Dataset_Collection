// Package http (delivery/http) — Gin router ve handler'ları.
//
// Clean Architecture'da bu katmanın görevi:
//   - HTTP isteklerini karşılamak
//   - İstek gövdesini DTO'ya dönüştürmek (binding)
//   - Use Case'i çağırmak
//   - Yanıtı JSON olarak göndermek
//   - HTTP hata kodlarını belirlemek
//
// Bu katman SADECE usecase paketini import eder.
// Domain entity'lerini ve repository'leri doğrudan kullanmaz.

package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ryuka/art-dataset-collector/internal/domain"
	"github.com/ryuka/art-dataset-collector/internal/usecase"
	"github.com/ryuka/art-dataset-collector/pkg/middleware"
)

// NewRouter, Gin engine'ini oluşturur ve tüm route'ları kaydeder.
//
// net/http karşılığı:
//
//	mux := http.NewServeMux()
//	mux.Handle("/api/images/next", middleware(imageHandler))
//	mux.Handle("/api/ratings", middleware(ratingHandler))
//
// Gin bunu çok daha temiz yapıyor: route gruplama, otomatik middleware chain.
func NewRouter(imageUC *usecase.ImageUseCase, ratingUC *usecase.RatingUseCase) *gin.Engine {
	// gin.New(): Boş engine — hiçbir middleware yok
	// gin.Default(): Logger ve Recovery middleware'leri otomatik ekler
	// Biz manuel ekleyeceğiz (daha fazla kontrol için)
	r := gin.New()

	// ── Global Middleware'ler ──────────────────────────────────
	// r.Use() → tüm route'lara uygulanır
	// net/http karşılığı: handler'ı wrap etmek zorunda kalırdın

	// Recovery: panic olursa 500 döner, sunucu çökmez
	// net/http karşılığı: defer recover() bloğu her handler'da tekrar
	r.Use(gin.Recovery())

	// Logger: Her isteği terminale yazar (yöntem, path, status, süre)
	// net/http karşılığı: manuel log.Printf veya logrus kurulumu
	r.Use(gin.Logger())

	// CORS: Tarayıcıdan gelen cross-origin isteklere izin ver
	r.Use(middleware.CORS())

	// ── Health Check (Auth gerektirmez) ──────────────────────
	// Yük dengeleyici (load balancer) veya Docker health check için
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "art-dataset-collector",
		})
	})

	// ── Frontend (Static Files) ───────────────────────────────
	// /       -> frontend/index.html
	// /admin  -> frontend/admin.html
	r.StaticFile("/", "./frontend/index.html")
	r.StaticFile("/admin", "./frontend/admin.html")

	// ── Handler'ları oluştur ──────────────────────────────────
	imageHandler := NewImageHandler(imageUC)
	ratingHandler := NewRatingHandler(ratingUC)
	adminHandler := NewAdminHandler(ratingUC)

	// ── API Route Grubu ───────────────────────────────────────
	// r.Group("/api") → tüm route'lar /api prefix'i alır
	// İkinci argüman middleware — sadece bu gruba uygulanır
	// net/http'de her route'u Manuel sarar veya prefix router kurardın
	api := r.Group("/api", middleware.APIKeyAuth())
	{
		// GET /api/images/next → Sonraki görseli getir
		api.GET("/images/next", imageHandler.GetNext)

		// GET /api/stats → İstatistikleri getir
		api.GET("/stats", imageHandler.GetStats)

		// POST /api/ratings → Puan kaydet
		api.POST("/ratings", ratingHandler.Create)

		// ── Admin Endpoints ───────────────────────────────────────
		api.GET("/admin/chart", adminHandler.GetChartData)
		api.GET("/admin/export", adminHandler.ExportCSV)
	}

	return r
}

// ──────────────────────────────────────────────────────────────
// YARDIMCI: Domain hatalarını HTTP koduna çevir
// ──────────────────────────────────────────────────────────────

// respondWithError, domain hatasına göre uygun HTTP durum kodu ile yanıt verir.
// Bu merkezi fonksiyon sayesinde her handler'da switch yazmak zorunda kalmayız.
func respondWithError(c *gin.Context, err error) {
	// errors.Is: hata zincirini kontrol eder (wrapped error'lar dahil)
	// net/http'de de aynı kullanım
	switch {
	case errors.Is(err, domain.ErrNotFound):
		// 404: Kayıt bulunamadı
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})

	case errors.Is(err, domain.ErrInvalidInput):
		// 400: Geçersiz girdi
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

	case errors.Is(err, domain.ErrNoImages):
		// 503: Veritabanında görsel yok, önce sync çalıştırılmalı
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": err.Error(),
			"hint":  "go run scripts/sync_drive.go komutunu çalıştırın",
		})

	default:
		// 500: Beklenmedik sunucu hatası
		// Detayı logluyoruz ama kullanıcıya göstermiyoruz (güvenlik)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "sunucu hatası",
		})
	}
}
