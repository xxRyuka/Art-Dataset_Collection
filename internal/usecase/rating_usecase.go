// Package usecase — Rating business logic ve DTO'ları.
//
// Bu dosyanın sorumluluğu:
//   1. Gelen CreateRatingRequest DTO'sunu doğrulamak (validasyon)
//   2. DTO'yu domain.Rating entity'sine dönüştürmek
//   3. Repository'yi çağırıp kaydetmek
//   4. Sonucu CreateRatingResponse DTO'suna dönüştürmek

package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/ryuka/art-dataset-collector/internal/domain"
)

// ──────────────────────────────────────────────────────────────
// REQUEST DTO: CreateRatingRequest
// POST /api/ratings body'sinden gelen veri.
// json tag'leri: JSON key adlarını belirler.
// binding tag'leri: Gin'in validasyon motoruna (validator/v10) direktif verir.
// ──────────────────────────────────────────────────────────────

// CreateRatingRequest, kullanıcının gönderdiği puan formunu temsil eder.
type CreateRatingRequest struct {
	// image_id: GET /api/images/next'den dönen UUID
	// binding:"required" → Gin: bu alan boş olamaz
	// net/http'de bunu manuel kontrol ederdin: if req.ImageID == "" { ... }
	ImageID string `json:"image_id" binding:"required"`

	// score: 1-10 arası puan
	// binding:"required,min=1,max=10" → Gin validatör kuralları
	// net/http'de: if score < 1 || score > 10 { ... }
	Score int `json:"score" binding:"required,min=1,max=10"`

	// age: Kullanıcının yaşı
	Age int `json:"age" binding:"required,min=1,max=120"`

	// gender: Cinsiyeti
	Gender string `json:"gender" binding:"required"`

	// city: Şehir (Türkçe karakter kabul edilir)
	City string `json:"city" binding:"required"`

	// knows_artist: Sanatçıyı tanıyor mu?
	// bool için binding:"required" gerekmiyor — false geçerli bir değer
	KnowsArtist bool `json:"knows_artist"`
}

// ──────────────────────────────────────────────────────────────
// RESPONSE DTO: CreateRatingResponse
// POST /api/ratings başarılı olduğunda dönen veri.
// ──────────────────────────────────────────────────────────────

// CreateRatingResponse, başarılı puan kaydının özeti.
type CreateRatingResponse struct {
	ID        string `json:"id"`         // Oluşturulan rating'in UUID'si
	ImageID   string `json:"image_id"`   // Hangi görsel puanlandı
	Score     int    `json:"score"`      // Verilen puan
	CreatedAt string `json:"created_at"` // Kayıt zamanı (ISO 8601)
}

// ──────────────────────────────────────────────────────────────
// USE CASE: RatingUseCase
// ──────────────────────────────────────────────────────────────

// RatingUseCase, puanlama ile ilgili tüm business logic'i içerir.
type RatingUseCase struct {
	ratingRepo domain.RatingRepository
}

// NewRatingUseCase, yeni bir RatingUseCase örneği oluşturur.
func NewRatingUseCase(ratingRepo domain.RatingRepository) *RatingUseCase {
	return &RatingUseCase{ratingRepo: ratingRepo}
}

// CreateRating, gelen isteği doğrular, kaydeder ve yanıt DTO'su döner.
func (uc *RatingUseCase) CreateRating(ctx context.Context, req *CreateRatingRequest) (*CreateRatingResponse, error) {
	// ── Ek Validasyon (Gin'in binding'i temel kontrolleri yapar) ──
	// Gin binding: required, min, max kontrollerini yapar.
	// Ama gender için "geçerli değer listesi" kontrolü business logic'e aittir.
	if err := uc.validateRequest(req); err != nil {
		return nil, err
	}

	// ── DTO → Domain Entity Dönüşümü ──
	// Request DTO'sunu veritabanına yazılacak entity'ye çevir.
	// Bu dönüşüm use case katmanında yapılır — ne handler ne repository yapar.
	rating := &domain.Rating{
		ImageID:     req.ImageID,
		Score:       req.Score,
		Age:         req.Age,
		Gender:      strings.ToLower(req.Gender), // Tutarlılık için küçük harfe çevir
		City:        strings.TrimSpace(req.City), // Baştaki/sondaki boşlukları temizle
		KnowsArtist: req.KnowsArtist,
	}

	// ── Repository Çağrısı ──
	// Create: rating INSERT + rating_count UPDATE (tek transaction)
	if err := uc.ratingRepo.Create(ctx, rating); err != nil {
		return nil, err
	}

	// ── Entity → Response DTO Dönüşümü ──
	// RETURNING ile dolan ID ve CreatedAt artık rating'de mevcut
	return &CreateRatingResponse{
		ID:        rating.ID,
		ImageID:   rating.ImageID,
		Score:     rating.Score,
		CreatedAt: rating.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), // ISO 8601
	}, nil
}

// validateRequest, Gin'in yapamadığı domain-spesifik validasyonları yapar.
func (uc *RatingUseCase) validateRequest(req *CreateRatingRequest) error {
	// Gender değeri geçerli listede mi?
	gender := strings.ToLower(req.Gender)
	if !domain.ValidGenders[gender] {
		return fmt.Errorf("%w: geçersiz cinsiyet değeri '%s' (geçerliler: male, female, other, prefer_not_to_say)",
			domain.ErrInvalidInput, req.Gender)
	}

	// Şehir en az 2 karakter olmalı
	if len(strings.TrimSpace(req.City)) < 2 {
		return fmt.Errorf("%w: şehir adı en az 2 karakter olmalı", domain.ErrInvalidInput)
	}

	// ImageID UUID formatında mı? (temel kontrol)
	if len(req.ImageID) != 36 {
		return fmt.Errorf("%w: image_id geçerli bir UUID olmalı", domain.ErrInvalidInput)
	}

	return nil
}
