// Package repository — Domain interface'lerinin PostgreSQL implementasyonları.
//
// Clean Architecture'da bu katmanın görevi:
//   - domain.ImageRepository interface'ini PostgreSQL ile somutlaştırmak
//   - SQL sorgularını çalıştırmak
//   - DB satırlarını domain entity'lerine (Image struct) dönüştürmek
//
// Bu katman SADECE domain paketini import eder.
// Gin, HTTP, veya business logic burada yoktur.

package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"               // PostgreSQL connection pool
	"github.com/ryuka/art-dataset-collector/internal/domain" // Domain entity'leri ve interface'ler
)

// imageRepository, domain.ImageRepository interface'ini implemente eder.
// Dışarıya açık değil (küçük harf) — sadece NewImageRepository ile oluşturulur.
type imageRepository struct {
	db *pgxpool.Pool // Bağlantı havuzu — her metot havuzdan bağlantı alır
}

// NewImageRepository, yeni bir imageRepository örneği oluşturur.
// domain.ImageRepository interface'ini döner — somut tipi gizler (Dependency Inversion).
//
// Kullanım (main.go'da):
//   var imageRepo domain.ImageRepository = repository.NewImageRepository(pool)
func NewImageRepository(db *pgxpool.Pool) domain.ImageRepository {
	return &imageRepository{db: db}
}

// ──────────────────────────────────────────────────────────────
// GetNextImage: En az puanlanan görseli getirir
// ──────────────────────────────────────────────────────────────

func (r *imageRepository) GetNextImage(ctx context.Context) (*domain.Image, error) {
	// SQL açıklaması:
	//   ORDER BY rating_count ASC → en az puanlananları öne al
	//   RANDOM()                  → aynı rating_count'lu olanlar arasında rastgele seç
	//   LIMIT 1                   → tek bir kayıt al
	//
	// İndeks: idx_images_rating_count bu sorguyu hızlandırır.
	const query = `
		SELECT id, drive_file_id, file_name, rating_count, created_at
		FROM images
		ORDER BY rating_count ASC, RANDOM()
		LIMIT 1
	`

	// QueryRow: Tek satır dönen sorgular için kullanılır
	// pgxpool.Pool.QueryRow → context, sql, args...
	row := r.db.QueryRow(ctx, query)

	img := &domain.Image{}
	err := row.Scan(
		// Scan: SQL kolonlarını sırasıyla struct field'larına atar
		&img.ID,
		&img.DriveFileID,
		&img.FileName,
		&img.RatingCount,
		&img.CreatedAt,
	)
	if err != nil {
		// pgx.ErrNoRows: Tablo boşsa bu hata döner
		// Biz bunu domain katmanındaki özel hata ile sarıyoruz
		return nil, fmt.Errorf("sonraki görsel alınamadı: %w", domain.ErrNoImages)
	}

	return img, nil
}

// ──────────────────────────────────────────────────────────────
// IncrementRatingCount: Puan sayacını atomik olarak artır
// ──────────────────────────────────────────────────────────────

func (r *imageRepository) IncrementRatingCount(ctx context.Context, imageID string) error {
	// "rating_count = rating_count + 1" → atomik güncelleme
	// Race condition yok: PostgreSQL satır kilidi bu işlemi güvenli kılar
	const query = `
		UPDATE images
		SET rating_count = rating_count + 1
		WHERE id = $1
	`

	// $1: pgx'te parametre işaretçisi (SQL injection'a karşı güvenli)
	// database/sql'de bu ? idi; pgx $1, $2... kullanır
	result, err := r.db.Exec(ctx, query, imageID)
	if err != nil {
		return fmt.Errorf("rating_count artırılamadı (image_id: %s): %w", imageID, err)
	}

	// Etkilenen satır sayısını kontrol et
	if result.RowsAffected() == 0 {
		return fmt.Errorf("image bulunamadı (id: %s): %w", imageID, domain.ErrNotFound)
	}

	return nil
}

// ──────────────────────────────────────────────────────────────
// BulkUpsert: Drive sync için toplu kayıt ekleme
// ──────────────────────────────────────────────────────────────

func (r *imageRepository) BulkUpsert(ctx context.Context, images []domain.Image) error {
	if len(images) == 0 {
		return nil // Eklenecek bir şey yok
	}

	// Tek tek INSERT yerine COPY kullanıyoruz — 10x daha hızlı
	// pgx'in CopyFrom özelliği: çok satırlı veriyi tek seferde gönderir
	// 14.000 dosya için çok önemli bir performans farkı yaratır
	_, err := r.db.CopyFrom(
		ctx,
		// Hedef tablo ve kolonlar
		[]string{"images"},                              // tablo adı (pgx bunu farklı alır)
		[]string{"drive_file_id", "file_name"},          // yazılacak kolonlar
		// Veri kaynağı: pgx.CopyFromRows yerine CopyFromSlice kullanıyoruz
		newImageCopySource(images),
	)
	if err != nil {
		// COPY başarısız olursa tek tek INSERT ile dene (fallback)
		return r.bulkUpsertFallback(ctx, images)
	}

	return nil
}

// bulkUpsertFallback, COPY başarısız olursa tek tek INSERT dener.
// ON CONFLICT DO NOTHING: Zaten var olan drive_file_id'leri atlar.
func (r *imageRepository) bulkUpsertFallback(ctx context.Context, images []domain.Image) error {
	const query = `
		INSERT INTO images (drive_file_id, file_name)
		VALUES ($1, $2)
		ON CONFLICT (drive_file_id) DO NOTHING
	`

	// Transaction başlat: ya hepsi başarılı olsun ya da hiçbiri
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("transaction başlatılamadı: %w", err)
	}
	// Fonksiyon bittiğinde hata varsa rollback yap
	// defer: Go'da kaynakları temizlemek için kullanılan deyim
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		}
	}()

	for _, img := range images {
		if _, err = tx.Exec(ctx, query, img.DriveFileID, img.FileName); err != nil {
			return fmt.Errorf("görsel kaydedilemedi (%s): %w", img.FileName, err)
		}
	}

	return tx.Commit(ctx)
}

// ──────────────────────────────────────────────────────────────
// GetStats: Özet istatistik
// ──────────────────────────────────────────────────────────────

func (r *imageRepository) GetStats(ctx context.Context) (*domain.ImageStats, error) {
	const query = `
		SELECT
			COUNT(*)                                  AS total_images,
			COUNT(*) FILTER (WHERE rating_count > 0)  AS rated_images,
			COUNT(*) FILTER (WHERE rating_count = 0)  AS unrated_images,
			COALESCE(SUM(rating_count), 0)            AS total_ratings
		FROM images
	`

	row := r.db.QueryRow(ctx, query)
	stats := &domain.ImageStats{}

	if err := row.Scan(
		&stats.TotalImages,
		&stats.RatedImages,
		&stats.UnratedImages,
		&stats.TotalRatings,
	); err != nil {
		return nil, fmt.Errorf("istatistikler alınamadı: %w", err)
	}

	return stats, nil
}

// ──────────────────────────────────────────────────────────────
// imageCopySource: pgx.CopyFromSource interface implementasyonu
// BulkUpsert'teki COPY işlemi için veri kaynağı sağlar.
// ──────────────────────────────────────────────────────────────

type imageCopySource struct {
	images []domain.Image
	index  int
}

func newImageCopySource(images []domain.Image) *imageCopySource {
	return &imageCopySource{images: images}
}

// Next: Bir sonraki satır var mı?
func (s *imageCopySource) Next() bool {
	return s.index < len(s.images)
}

// Values: Mevcut satırın değerlerini döner
func (s *imageCopySource) Values() ([]any, error) {
	img := s.images[s.index]
	s.index++
	return []any{img.DriveFileID, img.FileName}, nil
}

// Err: Okuma hatası var mı?
func (s *imageCopySource) Err() error {
	return nil
}
