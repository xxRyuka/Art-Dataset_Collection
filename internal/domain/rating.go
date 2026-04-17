// Package domain — Rating entity ve repository interface'i.
// Rating, bir kullanıcının bir görsele verdiği demografik puandır.

package domain

import (
	"context"
	"time"
)

// ──────────────────────────────────────────────────────────────
// ENTITY: Rating
// Veritabanındaki "ratings" tablosunun Go karşılığı.
// ──────────────────────────────────────────────────────────────

// Rating, tek bir kullanıcının tek bir görsele verdiği puanı ve
// demografik bilgilerini temsil eder.
// Kullanıcı hesabı/login yoktur; her "oturum" kendi verisini taşır.
type Rating struct {
	ID            string    `json:"id"`
	ImageID       string    `json:"image_id"`
	Score         int       `json:"score"`
	Age           int       `json:"age"`
	Gender        string    `json:"gender"`
	City          string    `json:"city"`
	KnowsArtist   bool      `json:"knows_artist"`
	FollowsArtist bool      `json:"follows_artist"`
	CreatedAt     time.Time `json:"created_at"`
}

// Gender için geçerli değerler — use case katmanında validasyonda kullanılır
var ValidGenders = map[string]bool{
	"male":              true,
	"female":            true,
	"other":             true,
	"prefer_not_to_say": true,
}

// ──────────────────────────────────────────────────────────────
// REPOSITORY INTERFACE: RatingRepository
// ──────────────────────────────────────────────────────────────

// RatingRepository, Rating entity'si için veri erişim sözleşmesidir.
type RatingRepository interface {
	// Create: Yeni bir puan kaydı oluşturur.
	// NOT: Bu metot aynı zamanda images.rating_count'u da 1 artırmalıdır.
	// Bu iki işlem tek bir transaction içinde gerçekleştirilir.
	Create(ctx context.Context, rating *Rating) error
	GetAllExports(ctx context.Context) ([]RatingExport, error)
	GetScoreDistribution(ctx context.Context) ([]ScoreDistribution, error)
}

// ─── ADMIN DASHBOARD DTOs ────────────────────────────────────────

// RatingExport, CSV'ye dışa aktarılacak satırın formatı
type RatingExport struct {
	FileName      string
	DriveFileID   string
	Score         int
	Age           int
	Gender        string
	City          string
	KnowsArtist   bool
	FollowsArtist bool
	CreatedAt     time.Time
}

// ScoreDistribution, 1-10 puan dağılım grafiği için kullanılır
type ScoreDistribution struct {
	Score int `json:"score"`
	Count int `json:"count"`
}

// ──────────────────────────────────────────────────────────────
// DOMAIN ERRORS
// Katmanlar arası hata iletişimi için özel hata tipleri.
// Bu sayede HTTP 404 veya HTTP 400 kararını use case katmanına bırakmayız.
// ──────────────────────────────────────────────────────────────

// ErrNotFound, istenen kayıt bulunamadığında kullanılır (HTTP 404).
var ErrNotFound = &DomainError{Code: "NOT_FOUND", Message: "kayıt bulunamadı"}

// ErrInvalidInput, gelen veri geçersiz olduğunda kullanılır (HTTP 400).
var ErrInvalidInput = &DomainError{Code: "INVALID_INPUT", Message: "geçersiz veri"}

// ErrNoImages, veritabanında hiç görsel yokken kullanılır.
var ErrNoImages = &DomainError{Code: "NO_IMAGES", Message: "veritabanında henüz görsel yok, önce sync çalıştırın"}

// DomainError, tüm domain hataları için ortak yapı.
type DomainError struct {
	Code    string // Makine tarafından okunabilir hata kodu
	Message string // İnsan tarafından okunabilir açıklama
}

// Error(), Go'nun error interface'ini karşılar.
func (e *DomainError) Error() string {
	return e.Message
}
