// sync_drive/main.go — Google Drive → PostgreSQL senkronizasyon scripti.
//
// Bu script tek seferlik (veya periyodik) çalıştırılır.
// API sunucusu değil, bir CLI aracıdır.
//
// Çalıştırma:
//   go run ./scripts/sync_drive
//
// Ne yapar?
//   1. .env dosyasını yükler
//   2. Drive'daki hedef klasörü sayfalandırmalı tarar
//   3. Tüm dosya ID ve adlarını PostgreSQL'e kaydeder
//   4. Zaten kayıtlı olanları atlar (idempotent — tekrar çalıştırılabilir)
//
// 14.000 dosya için tahmini süre: 2-3 dakika

package main

import (
	"context"
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/ryuka/art-dataset-collector/internal/domain"
	"github.com/ryuka/art-dataset-collector/internal/repository"
	"github.com/ryuka/art-dataset-collector/pkg/config"
	"github.com/ryuka/art-dataset-collector/pkg/database"
	"github.com/ryuka/art-dataset-collector/pkg/drive"
)

func main() {
	// .env dosyasını yükle
	if err := godotenv.Load(); err != nil {
		log.Println("Uyarı: .env dosyası bulunamadı, sistem ortam değişkenleri kullanılıyor")
	}

	// Yapılandırmayı yükle
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Yapılandırma yüklenemedi: %v", err)
	}

	// Kontrol: Drive bilgileri tanımlı mı?
	if cfg.DriveFolderID == "" {
		log.Fatal("GOOGLE_DRIVE_FOLDER_ID .env'de tanımlı değil")
	}
	if cfg.GoogleAPIKey == "" {
		log.Fatal("GOOGLE_API_KEY .env'de tanımlı değil\n" +
			"Almak için: console.cloud.google.com → APIs & Services → Credentials → API Key")
	}

	ctx := context.Background()

	// ── Veritabanı Bağlantısı ────────────────────────────────
	pool, err := database.NewPool(ctx, cfg.DSN())
	if err != nil {
		log.Fatalf("Veritabanına bağlanılamadı: %v", err)
	}
	defer pool.Close()

	// Migration: Tablolar yoksa oluştur
	if err := database.RunMigrations(ctx, pool); err != nil {
		log.Fatalf("Migrasyon başarısız: %v", err)
	}

	imageRepo := repository.NewImageRepository(pool)

	// ── Drive İstemcisi ───────────────────────────────────────────
	// Klasör herkese açık olduğu için Google API Key yeterlidir.
	// credentials.json veya Service Account gerekmez.
	driveClient, err := drive.NewClient(ctx, cfg.GoogleAPIKey, cfg.DriveFolderID)
	if err != nil {
		log.Fatalf("Drive istemcisi oluşturulamadı: %v", err)
	}

	// ── Dosyaları Listele ─────────────────────────────────────
	log.Printf("Drive klasörü taranıyor (klasör ID: %s)...", cfg.DriveFolderID)
	startTime := time.Now()

	allFiles, err := driveClient.ListAllFiles(ctx)
	if err != nil {
		log.Fatalf("Drive dosyaları listelenemedi: %v", err)
	}

	log.Printf("Toplam %d dosya bulundu (%.1f saniye)", len(allFiles), time.Since(startTime).Seconds())

	if len(allFiles) == 0 {
		log.Println("Klasörde dosya yok. Klasör ID'sini ve paylaşım yetkilerini kontrol edin.")
		return
	}

	// ── Veritabanına Toplu Kaydet ─────────────────────────────
	// Drive dosyalarını domain.Image slice'ına dönüştür
	images := make([]domain.Image, 0, len(allFiles))
	for _, f := range allFiles {
		images = append(images, domain.Image{
			DriveFileID: f.ID,
			FileName:    f.Name,
		})
	}

	log.Printf("%d görsel veritabanına kaydediliyor...", len(images))
	saveStart := time.Now()

	// BulkUpsert: Zaten kayıtlı olanlar atlanır (ON CONFLICT DO NOTHING)
	// Bu script'i tekrar çalıştırsanız yeni dosyalar eklenir; eskiler güncellenmez.
	if err := imageRepo.BulkUpsert(ctx, images); err != nil {
		log.Fatalf("Görseller kaydedilemedi: %v", err)
	}

	// ── Özet ─────────────────────────────────────────────────
	stats, _ := imageRepo.GetStats(ctx)

	log.Printf("───────────────────────────────────────")
	log.Printf("✓ Senkronizasyon tamamlandı!")
	log.Printf("  Toplam süre    : %.1f saniye", time.Since(startTime).Seconds())
	log.Printf("  Kaydetme süresi: %.1f saniye", time.Since(saveStart).Seconds())
	log.Printf("  Toplam görsel  : %d", stats.TotalImages)
	log.Printf("  Puanlanmış     : %d", stats.RatedImages)
	log.Printf("  Puanlanmamış   : %d", stats.UnratedImages)
	log.Printf("───────────────────────────────────────")
	log.Printf("Sonraki adım: go run cmd/api/main.go")
}
