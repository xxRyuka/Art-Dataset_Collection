// Package repository — Rating entity'si için PostgreSQL implementasyonu.
//
// Kritik davranış: Rating oluşturma ve rating_count artırma
// tek bir transaction içinde gerçekleşir.
// Biri başarısız olursa her ikisi de geri alınır → veri tutarlılığı.

package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ryuka/art-dataset-collector/internal/domain"
)

// ratingRepository, domain.RatingRepository interface'ini implemente eder.
type ratingRepository struct {
	db *pgxpool.Pool
}

// NewRatingRepository, yeni bir ratingRepository örneği oluşturur.
// domain.RatingRepository interface'ini döner — somut tip gizlenir.
func NewRatingRepository(db *pgxpool.Pool) domain.RatingRepository {
	return &ratingRepository{db: db}
}

// Create, yeni bir puan kaydı oluşturur VE aynı transaction'da
// ilgili görselin rating_count'unu 1 artırır.
//
// İki ayrı sorgu yerine transaction kullanmamızın sebebi:
//
//	Senaryo: Rating INSERT başarılı, ama sonraki UPDATE başarısız.
//	Sonuç olmadan: Rating kaydı var ama sayaç güncellenmedi → veri tutarsızlığı.
//	Transaction ile: Her ikisi de başarılı olur veya her ikisi de geri alınır.
func (r *ratingRepository) Create(ctx context.Context, rating *domain.Rating) error {
	// Transaction başlat
	// net/http projelerinde de aynı: db.BeginTx(ctx, nil)
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("transaction başlatılamadı: %w", err)
	}

	// defer: Fonksiyon bitişinde (başarı veya hata) çalışır.
	// Hata varsa rollback yap; başarılıysa commit zaten yapıldı, rollback etkisiz kalır.
	defer tx.Rollback(ctx) // Commit başarılıysa bu çağrı zararsız olur

	// 1. ADIM: Rating kaydını ekle
	const insertRating = `
		INSERT INTO ratings (image_id, score, age, gender, city, knows_artist, follows_artist)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`
	// RETURNING: INSERT edilen satırın belirtilen kolonlarını geri döner
	// Bu sayede ayrı bir SELECT yapmadan id ve created_at'ı alırız
	row := tx.QueryRow(ctx, insertRating,
		rating.ImageID,
		rating.Score,
		rating.Age,
		rating.Gender,
		rating.City,
		rating.KnowsArtist,
		rating.FollowsArtist,
	)

	// Dönen değerleri struct'a yaz
	if err := row.Scan(&rating.ID, &rating.CreatedAt); err != nil {
		return fmt.Errorf("rating kaydedilemedi: %w", err)
	}

	// 2. ADIM: Görselin rating_count'unu artır (aynı transaction içinde)
	const updateCount = `
		UPDATE images
		SET rating_count = rating_count + 1
		WHERE id = $1
	`
	result, err := tx.Exec(ctx, updateCount, rating.ImageID)
	if err != nil {
		return fmt.Errorf("rating_count güncellenemedi: %w", err)
	}

	// Etkilenen satır yoksa görsel mevcut değil demektir
	if result.RowsAffected() == 0 {
		return fmt.Errorf("image bulunamadı (id: %s): %w", rating.ImageID, domain.ErrNotFound)
	}

	// 3. ADIM: Her iki işlem de başarılı → transaction'ı onayla
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("transaction onaylanamadı: %w", err)
	}

	return nil
}

// GetAllExports, JOIN işlemi yaparak CSV'ye basılacak bilgileri getirir.
func (r *ratingRepository) GetAllExports(ctx context.Context) ([]domain.RatingExport, error) {
	const query = `
		SELECT
			i.file_name,
			i.drive_file_id,
			r.score,
			r.age,
			r.gender,
			r.city,
			r.knows_artist,
			r.follows_artist,
			r.created_at
		FROM ratings r
		JOIN images i ON i.id = r.image_id
		ORDER BY r.created_at ASC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("export sorgusu başarısız: %w", err)
	}
	defer rows.Close()

	var exports []domain.RatingExport
	for rows.Next() {
		var e domain.RatingExport
		if err := rows.Scan(
			&e.FileName,
			&e.DriveFileID,
			&e.Score,
			&e.Age,
			&e.Gender,
			&e.City,
			&e.KnowsArtist,
			&e.FollowsArtist,
			&e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("export satırı okunamadı: %w", err)
		}
		exports = append(exports, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("export satırları okunurken hata: %w", err)
	}
	return exports, nil
}

// GetDashboardStats, tüm admin dashboard metriklerini tek seferde hesaplar.
func (r *ratingRepository) GetDashboardStats(ctx context.Context) (*domain.DashboardStats, error) {
	stats := &domain.DashboardStats{}

	// 1. Toplam Puanlama Sayısı, Knows/Follows sayıları puan başına (her kayıt bir puanlamadır, dolayısıyla toplam rating sayısı)
	// NOT: "Kac kisi katilmis" sorusunun tam cevabını bulmak cookie tabanlı olmadığı için zordur ancak basitçe toplam satır / 10 veya toplam tekil kombinasyon olabilir.
	// Biz burada oyların %10'u gibi bir kaba tahmin veya sadece total_ratings göstereceğiz. "Session" ID olmadığı için, TotalRatings üzerinden gösterim yapılır.
	const summaryQuery = `
		SELECT 
			COUNT(*) as total_ratings,
			COUNT(*) FILTER (WHERE knows_artist = TRUE) as knows_artist_count,
			COUNT(*) FILTER (WHERE follows_artist = TRUE) as follows_artist_count
		FROM ratings
	`
	err := r.db.QueryRow(ctx, summaryQuery).Scan(
		&stats.TotalRatings,
		&stats.KnowsArtistCount,
		&stats.FollowsArtistCount,
	)
	if err != nil {
		return nil, fmt.Errorf("özet istatistikleri alınamadı: %w", err)
	}

	// 13 oylama yapan 1 kişi ise yaklaşık kişi sayısı
	stats.TotalParticipants = stats.TotalRatings / 10

	// 2. Puan dağılımı
	const scoreQuery = `
		SELECT score, COUNT(*) as count
		FROM ratings
		GROUP BY score
		ORDER BY score ASC
	`
	rows, err := r.db.Query(ctx, scoreQuery)
	if err != nil {
		return nil, fmt.Errorf("puan dağılımı alınamadı: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var s domain.ScoreDistribution
		if err := rows.Scan(&s.Score, &s.Count); err != nil {
			return nil, err
		}
		stats.ScoreDistribution = append(stats.ScoreDistribution, s)
	}
	rows.Close()

	// 3. Cinsiyet dağılımı
	const genderQuery = `
		SELECT gender, COUNT(*) as count
		FROM ratings
		GROUP BY gender
		ORDER BY count DESC
	`
	gRows, err := r.db.Query(ctx, genderQuery)
	if err != nil {
		return nil, fmt.Errorf("cinsiyet dağılımı alınamadı: %w", err)
	}
	defer gRows.Close()

	for gRows.Next() {
		var g domain.GenderDistribution
		if err := gRows.Scan(&g.Gender, &g.Count); err != nil {
			return nil, err
		}
		stats.GenderDistribution = append(stats.GenderDistribution, g)
	}

	return stats, nil
}
