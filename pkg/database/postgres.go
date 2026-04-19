// Package database — PostgreSQL bağlantı havuzunu (connection pool) yönetir.
// pgxpool kullanıyoruz: aynı anda birden fazla istek geldiğinde her biri
// havuzdan bir bağlantı alır, işini bitirince geri bırakır.
// Bu, her istek için yeni bağlantı açmaktan çok daha verimlidir.

package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool" // PostgreSQL connection pool
)

// NewPool, verilen DSN (Data Source Name) ile bir PostgreSQL bağlantı havuzu açar.
// Başarısız olursa hata döner — uygulama DB olmadan başlamamalı.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	// pgxpool.ParseConfig: DSN string'ini yapılandırma nesnesine dönüştürür
	// net/http eşdeğeri: sql.Open("postgres", dsn) — ama pgxpool daha güçlü
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("veritabanı yapılandırması okunamadı: %w", err)
	}

	// Connection pool ayarları
	cfg.MaxConns = 20                       // Aynı anda en fazla 20 bağlantı
	cfg.MinConns = 2                        // Her zaman en az 2 bağlantı hazır tutsun
	cfg.MaxConnLifetime = 1 * time.Hour     // Bağlantı en fazla 1 saat yaşasın
	cfg.MaxConnIdleTime = 30 * time.Minute  // 30 dakika boştaysa kapat
	cfg.HealthCheckPeriod = 1 * time.Minute // Her dakika bağlantıyı sağlık kontrolüne sok

	// Havuzu oluştur ve bağlantıyı test et
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("veritabanı bağlantı havuzu açılamadı: %w", err)
	}

	// Ping ile gerçek bağlantıyı doğrula
	// net/http projelerinde de db.PingContext(ctx) ile aynı amaç
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("veritabanına ping atılamadı: %w", err)
	}

	return pool, nil
}

// RunMigrations, migrations/ klasöründeki SQL dosyalarını sırayla çalıştırır.
// Üretim ortamında golang-migrate gibi bir kütüphane tercih edilir,
// ama bu proje için basit bir implementasyon yeterlidir.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	// Migration SQL'leri burada tanımlanır.
	// Gerçek projede ayrı .sql dosyalarından okunabilir.
	migrations := []string{
		// 001: images tablosu
		`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

		CREATE TABLE IF NOT EXISTS images (
			id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			drive_file_id VARCHAR(255) NOT NULL UNIQUE,
			file_name     VARCHAR(500) NOT NULL,
			rating_count  INTEGER NOT NULL DEFAULT 0,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		-- En az puanlananı hızlı bulmak için index (ORDER BY rating_count ASC)
		CREATE INDEX IF NOT EXISTS idx_images_rating_count ON images(rating_count ASC);`,

		// 002: ratings tablosu
		`CREATE TABLE IF NOT EXISTS ratings (
			id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			session_id     UUID NOT NULL,
			image_id       UUID NOT NULL REFERENCES images(id) ON DELETE CASCADE,
			score          SMALLINT NOT NULL CHECK (score BETWEEN 1 AND 10),
			age            SMALLINT NOT NULL CHECK (age > 0 AND age < 120),
			gender         VARCHAR(30) NOT NULL,
			city           VARCHAR(150) NOT NULL,
			knows_artist   BOOLEAN NOT NULL DEFAULT FALSE,
			follows_artist BOOLEAN NOT NULL DEFAULT FALSE,
			created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		-- image_id'ye göre sorgu hızını artırmak için index
		CREATE INDEX IF NOT EXISTS idx_ratings_image_id ON ratings(image_id);

`,

		// 003: existing database update
		`ALTER TABLE ratings ADD COLUMN IF NOT EXISTS follows_artist BOOLEAN NOT NULL DEFAULT FALSE;`,

		// 004: session_id ekle — mevcut veritabanları için
		`ALTER TABLE ratings ADD COLUMN IF NOT EXISTS session_id UUID;
		CREATE INDEX IF NOT EXISTS idx_ratings_session_id ON ratings(session_id);`,
	}

	for i, sql := range migrations {
		if _, err := pool.Exec(ctx, sql); err != nil {
			return fmt.Errorf("migration %d çalıştırılamadı: %w", i+1, err)
		}
	}

	return nil
}
