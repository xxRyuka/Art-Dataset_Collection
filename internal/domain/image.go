// Package domain — Clean Architecture'ın en iç katmanı.
// Bu pakette SADECE:
//   - Entity'ler (veri yapıları / struct'lar)
//   - Repository interface'leri (sözleşmeler)
//   - Use Case interface'leri (sözleşmeler)
// vardır. Hiçbir dış paket (gin, pgx, vb.) import edilmez.
// Bu sayede domain katmanı tamamen bağımsız ve test edilebilir kalır.

package domain

import (
	"context"
	"time"
)

// ──────────────────────────────────────────────────────────────
// ENTITY: Image
// Veritabanındaki "images" tablosunun Go karşılığı.
// Her alan, tablodaki bir kolona karşılık gelir.
// ──────────────────────────────────────────────────────────────

// Image, Google Drive'daki bir görseli temsil eder.
// Fiziksel dosya Drive'da durur; biz sadece metadata'yı tutarız.
type Image struct {
	ID          string    // PostgreSQL UUID — PRIMARY KEY
	DriveFileID string    // Drive'daki dosyanın ID'si (thumbnail URL için kullanılır)
	FileName    string    // Dosyanın orijinal adı (CSV export'ta görünür)
	RatingCount int       // Kaç kez puanlandı (dağılım algoritması için)
	CreatedAt   time.Time // Kaydın oluşturulma zamanı
}

// ──────────────────────────────────────────────────────────────
// REPOSITORY INTERFACE: ImageRepository
// "Repository Pattern": Veri katmanı ile business logic arasındaki sözleşme.
// Use Case katmanı bu interface'i kullanır; PostgreSQL implementasyonunu bilmez.
// Test sırasında bu interface'i mock'layabiliriz.
// ──────────────────────────────────────────────────────────────

// ImageRepository, Image entity'si için veri erişim sözleşmesidir.
type ImageRepository interface {
	// GetNextImage: En az puanlanan görseli getirir.
	// SQL: SELECT * FROM images ORDER BY rating_count ASC, RANDOM() LIMIT 1
	GetNextImage(ctx context.Context) (*Image, error)

	// IncrementRatingCount: Bir görselin puan sayacını atomik olarak 1 artırır.
	// SQL: UPDATE images SET rating_count = rating_count + 1 WHERE id = $1
	IncrementRatingCount(ctx context.Context, imageID string) error

	// BulkUpsert: Drive sync scriptinin toplu kayıt için kullandığı metot.
	// Zaten varsa güncelleme yapmaz (idempotent).
	// SQL: INSERT ... ON CONFLICT (drive_file_id) DO NOTHING
	BulkUpsert(ctx context.Context, images []Image) error

	// GetStats: Özet istatistik döner (kaç görsel var, kaç puanlanmadı).
	GetStats(ctx context.Context) (*ImageStats, error)
}

// ImageStats, /api/stats endpoint'i için özet bilgi taşır.
type ImageStats struct {
	TotalImages   int // Toplam görsel sayısı
	RatedImages   int // En az 1 kez puanlanmış görsel sayısı
	UnratedImages int // Hiç puanlanmamış görsel sayısı
	TotalRatings  int // Toplam puan sayısı
}
