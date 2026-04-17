// Package usecase — Business logic katmanı.
//
// Clean Architecture'da bu katmanın görevi:
//   - Gelen DTO'ları (Data Transfer Object) doğrulamak
//   - Domain repository'leri çağırmak
//   - Sonucu Response DTO'ya dönüştürmek
//   - HTTP ve DB'den bağımsız kalmak (gin veya pgx import edilmez)
//
// DTO nedir?
//   Data Transfer Object: Katmanlar arası veri taşıyan yapılar.
//   - Request DTO: HTTP'den gelen ham veri (JSON body)
//   - Response DTO: HTTP'ye gönderilecek şekillendirilmiş veri
//   Domain entity (Image, Rating) ile DTO FARKLIDIR:
//   - Entity: Veritabanı odaklı, tüm alanları içerir
//   - DTO: Sadece ilgili katmanın ihtiyacı olan alanları içerir

package usecase

import (
	"context"
	"fmt"

	"github.com/ryuka/art-dataset-collector/internal/domain"
)

// ──────────────────────────────────────────────────────────────
// RESPONSE DTO: NextImageResponse
// GET /api/images/next endpoint'inin döneceği veri yapısı.
// domain.Image'dan farklı: thumbnail URL hesaplanmış olarak eklenir.
// ──────────────────────────────────────────────────────────────

// NextImageResponse, frontend'e gönderilecek görsel bilgisini taşır.
type NextImageResponse struct {
	ImageID      string `json:"image_id"`      // UUID — rating POST'unda kullanılacak
	ThumbnailURL string `json:"thumbnail_url"` // Drive thumbnail linki
	FileName     string `json:"file_name"`     // Referans için dosya adı
	RatingCount  int    `json:"rating_count"`  // Debug için: kaç kez puanlandı
}

// StatsResponse, GET /api/stats endpoint'inin döneceği istatistikler.
type StatsResponse struct {
	TotalImages   int     `json:"total_images"`
	RatedImages   int     `json:"rated_images"`
	UnratedImages int     `json:"unrated_images"`
	TotalRatings  int     `json:"total_ratings"`
	CompletionPct float64 `json:"completion_pct"` // Yüzde tamamlanma
}

// ──────────────────────────────────────────────────────────────
// USE CASE: ImageUseCase
// ──────────────────────────────────────────────────────────────

// ImageUseCase, Image ile ilgili tüm business logic'i içerir.
type ImageUseCase struct {
	// domain.ImageRepository interface — somut tip değil
	// Bu sayede test sırasında mock repository geçirebiliriz
	imageRepo domain.ImageRepository
}

// NewImageUseCase, yeni bir ImageUseCase örneği oluşturur.
// Dependency Injection: Repository dışarıdan verilir, içeride oluşturulmaz.
func NewImageUseCase(imageRepo domain.ImageRepository) *ImageUseCase {
	return &ImageUseCase{imageRepo: imageRepo}
}

// GetNextImage, kullanıcıya gösterilecek sonraki görseli hazırlar.
// İş kuralı: En az puanlanan görseli seç, Drive thumbnail URL'ini oluştur.
func (uc *ImageUseCase) GetNextImage(ctx context.Context) (*NextImageResponse, error) {
	// Repository'den entity'yi al
	img, err := uc.imageRepo.GetNextImage(ctx)
	if err != nil {
		// Hata olduğu gibi döner — handler katmanı HTTP koduna çevirir
		return nil, err
	}

	// ── Business Logic: Thumbnail URL oluşturma ──────────────
	// Drive'daki görseli doğrudan indirmek yerine thumbnail endpoint'ini kullanıyoruz.
	// sz=w1000: 1000px genişlik — bant genişliğinden tasarruf
	// Daha küçük: sz=w500, daha büyük: sz=w2000
	thumbnailURL := fmt.Sprintf(
		"https://drive.google.com/thumbnail?id=%s&sz=w1000",
		img.DriveFileID,
	)

	// Domain entity → Response DTO dönüşümü
	return &NextImageResponse{
		ImageID:      img.ID,
		ThumbnailURL: thumbnailURL,
		FileName:     img.FileName,
		RatingCount:  img.RatingCount,
	}, nil
}

// GetStats, özet istatistikleri hesaplar ve döner.
func (uc *ImageUseCase) GetStats(ctx context.Context) (*StatsResponse, error) {
	stats, err := uc.imageRepo.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	// Tamamlanma yüzdesini hesapla (0 bölme koruması)
	var completionPct float64
	if stats.TotalImages > 0 {
		completionPct = float64(stats.RatedImages) / float64(stats.TotalImages) * 100
	}

	return &StatsResponse{
		TotalImages:   stats.TotalImages,
		RatedImages:   stats.RatedImages,
		UnratedImages: stats.UnratedImages,
		TotalRatings:  stats.TotalRatings,
		CompletionPct: completionPct,
	}, nil
}
