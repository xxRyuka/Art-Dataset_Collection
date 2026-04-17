// export_csv.go — Veri seti CSV export scripti.
//
// Veri toplama tamamlandıktan sonra bu scripti çalıştırarak
// AI eğitimi için kullanılacak CSV dosyasını üret.
//
// Çalıştırma:
//   go run scripts/export_csv.go
//
// Çıktı: dataset_export_YYYYMMDD_HHMMSS.csv
//
// CSV Kolonları:
//   file_name, drive_file_id, score, age, gender, city, knows_artist, rated_at

package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/ryuka/art-dataset-collector/pkg/config"
	"github.com/ryuka/art-dataset-collector/pkg/database"
)

func main() {
	// .env dosyasını yükle
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Yapılandırma yüklenemedi: %v", err)
	}

	ctx := context.Background()

	// Veritabanı bağlantısı
	pool, err := database.NewPool(ctx, cfg.DSN())
	if err != nil {
		log.Fatalf("Veritabanına bağlanılamadı: %v", err)
	}
	defer pool.Close()

	// ── JOIN Sorgusu ──────────────────────────────────────────
	// images ve ratings tablolarını birleştir
	// Her puan kaydı için görselin adını da ekle
	const query = `
		SELECT
			i.file_name,
			i.drive_file_id,
			r.score,
			r.age,
			r.gender,
			r.city,
			r.knows_artist,
			r.created_at
		FROM ratings r
		JOIN images i ON i.id = r.image_id
		ORDER BY r.created_at ASC
	`

	log.Println("Veriler veritabanından çekiliyor...")

	rows, err := pool.Query(ctx, query)
	if err != nil {
		log.Fatalf("Sorgu başarısız: %v", err)
	}
	defer rows.Close()

	// ── CSV Dosyası Oluştur ───────────────────────────────────
	// Dosya adına tarih-saat ekle: istenen zaman tekrar çalıştırılabilir
	timestamp := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("dataset_export_%s.csv", timestamp)

	file, err := os.Create(fileName)
	if err != nil {
		log.Fatalf("CSV dosyası oluşturulamadı: %v", err)
	}
	defer file.Close()

	// CSV writer: encoding/csv Go standart kütüphanesidir, dış bağımlılık yok
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Başlık satırı
	headers := []string{
		"file_name",
		"drive_file_id",
		"score",
		"age",
		"gender",
		"city",
		"knows_artist",
		"rated_at",
	}
	if err := writer.Write(headers); err != nil {
		log.Fatalf("Başlık yazılamadı: %v", err)
	}

	// ── Satırları Yaz ─────────────────────────────────────────
	rowCount := 0

	for rows.Next() {
		var (
			fileName     string
			driveFileID  string
			score        int
			age          int
			gender       string
			city         string
			knowsArtist  bool
			createdAt    time.Time
		)

		// rows.Scan: Her veritabanı sütununu karşılık gelen değişkene ata
		if err := rows.Scan(
			&fileName,
			&driveFileID,
			&score,
			&age,
			&gender,
			&city,
			&knowsArtist,
			&createdAt,
		); err != nil {
			log.Printf("Satır okunamadı: %v", err)
			continue
		}

		// bool → "true"/"false" metne çevir (CSV için)
		knowsArtistStr := "false"
		if knowsArtist {
			knowsArtistStr = "true"
		}

		record := []string{
			fileName,
			driveFileID,
			strconv.Itoa(score),
			strconv.Itoa(age),
			gender,
			city,
			knowsArtistStr,
			createdAt.Format("2006-01-02T15:04:05Z"),
		}

		if err := writer.Write(record); err != nil {
			log.Fatalf("Satır yazılamadı: %v", err)
		}

		rowCount++

		// Her 1000 satırda bir ilerleme mesajı
		if rowCount%1000 == 0 {
			log.Printf("  %d satır işlendi...", rowCount)
		}
	}

	// Rows hata kontrolü (döngü bittikten sonra)
	if err := rows.Err(); err != nil {
		log.Fatalf("Satır okuma hatası: %v", err)
	}

	log.Printf("───────────────────────────────────────")
	log.Printf("✓ CSV export tamamlandı!")
	log.Printf("  Dosya    : %s", fileName)
	log.Printf("  Toplam   : %d puan kaydı", rowCount)
	log.Printf("───────────────────────────────────────")
}
