// main.go — Uygulamanın giriş noktası.
//
// Clean Architecture'da main.go'nun tek sorumluluğu:
//   1. Yapılandırmayı yükle (Config)
//   2. Tüm bağımlılıkları oluştur (DB, Repository, UseCase, Handler)
//   3. Bunları birbirine "tel" (wire) et — Dependency Injection
//   4. Sunucuyu başlat
//
// Bu dosya "Composition Root" olarak adlandırılır:
// Tüm somut bağımlılıklar burada oluşturulur ve enjekte edilir.
// Başka hiçbir yerde somut Repository veya UseCase oluşturulmaz.

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryuka/art-dataset-collector/internal/repository"
	deliveryHTTP "github.com/ryuka/art-dataset-collector/internal/delivery/http"
	"github.com/ryuka/art-dataset-collector/internal/usecase"
	"github.com/ryuka/art-dataset-collector/pkg/config"
	"github.com/ryuka/art-dataset-collector/pkg/database"
)

func main() {
	// ── 1. YAPILANDIRMAYI YÜKLE ───────────────────────────────
	// .env dosyasını oku, zorunlu değerleri kontrol et
	cfg, err := config.Load()
	if err != nil {
		// log.Fatalf: Mesajı yazar ve os.Exit(1) çağırır — uygulama başlamaz
		log.Fatalf("Yapılandırma yüklenemedi: %v", err)
	}

	// Gin çalışma modunu ayarla (debug → hem panikleri hem de route listesini gösterir)
	// release modunda daha az log, daha yüksek performans
	gin.SetMode(cfg.GinMode)

	log.Printf("Uygulama başlatılıyor... (mod: %s, port: %s)", cfg.GinMode, cfg.Port)

	// ── 2. VERİTABANI BAĞLANTISI ─────────────────────────────
	// Context: 10 saniye içinde bağlanamazsa iptal et
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Bağlantı havuzunu oluştur
	pool, err := database.NewPool(ctx, cfg.DSN())
	if err != nil {
		log.Fatalf("Veritabanına bağlanılamadı: %v", err)
	}
	// defer: main() bittiğinde bağlantıları kapat (graceful shutdown)
	defer pool.Close()

	log.Println("Veritabanı bağlantısı başarılı ✓")

	// ── 3. MİGRASYONLARI ÇALIŞTIR ────────────────────────────
	// Tablolar yoksa oluşturulur; varsa dokunulmaz (IF NOT EXISTS)
	if err := database.RunMigrations(context.Background(), pool); err != nil {
		log.Fatalf("Migrasyon başarısız: %v", err)
	}

	log.Println("Veritabanı migrasyonları tamamlandı ✓")

	// ── 4. BAĞIMLILIKLARI TEL ET (DEPENDENCY INJECTION) ──────
	// Aşağıdan yukarıya doğru oluşturuyoruz:
	//   Repository → UseCase → Handler → Router

	// Repository katmanı (PostgreSQL implementasyonları)
	imageRepo := repository.NewImageRepository(pool)
	ratingRepo := repository.NewRatingRepository(pool)

	// Use Case katmanı (business logic)
	imageUC := usecase.NewImageUseCase(imageRepo)
	ratingUC := usecase.NewRatingUseCase(ratingRepo)

	// Router (Gin engine + handler'lar)
	router := deliveryHTTP.NewRouter(imageUC, ratingUC)

	// ── 5. HTTP SUNUCUSUNU BAŞLAT ─────────────────────────────
	// Gin'i doğrudan r.Run() ile başlatmak yerine net/http.Server kullanıyoruz.
	// Sebebi: Graceful shutdown (bekleyen istekler bitmeden kapanma) için
	// net/http.Server.Shutdown() gerekiyor.
	//
	// r.Run(":8080") → kısayol, ama graceful shutdown desteklemez
	// net/http.Server → tam kontrol
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router, // Gin engine, http.Handler interface'ini implemente eder
		ReadTimeout:  15 * time.Second, // İstek okuma zaman aşımı
		WriteTimeout: 15 * time.Second, // Yanıt yazma zaman aşımı
		IdleTimeout:  60 * time.Second, // Keep-alive bağlantılar için
	}

	// Sunucuyu ayrı bir goroutine'de başlat (main thread'i bloklamasın)
	// Goroutine: Go'nun hafif iş parçacığı (thread değil, çok daha ucuz)
	go func() {
		log.Printf("Sunucu başlatıldı → http://localhost:%s", cfg.Port)
		log.Printf("Sağlık kontrolü → http://localhost:%s/health", cfg.Port)

		// ListenAndServe: Sunucuyu başlatır, hata olana dek çalışır
		// Hata: Kapatma komutu veya port meşgul gibi durumlar
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Sunucu hatası: %v", err)
		}
	}()

	// ── 6. GRACEFUL SHUTDOWN ──────────────────────────────────
	// OS sinyalini bekle: Ctrl+C (SIGINT) veya kill (SIGTERM)
	// Bu sayede Kubernetes veya Docker gibi ortamlarda düzgünce kapanır.
	quit := make(chan os.Signal, 1)
	// signal.Notify: Os sinyallerini quit kanalına yönlendir
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit // Sinyal gelene kadar burada bekle (blokla)

	log.Println("Kapatma sinyali alındı, sunucu 30 saniye içinde kapanacak...")

	// 30 saniye içinde bekleyen istekleri tamamla, sonra kapat
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Sunucu zorla kapatıldı: %v", err)
	}

	log.Println("Sunucu başarıyla kapatıldı. İyi günler!")
}
