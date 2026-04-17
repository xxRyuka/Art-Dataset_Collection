// Package config — Ortam değişkenlerini (.env) okuyup yapılandırma struct'ına dönüştürür.
// Bu paket, uygulama boyunca paylaşılan tek bir Config örneği sağlar.
// Tüm ortam değişkenleri buradan okunur; başka hiçbir pakette os.Getenv() çağrılmaz.

package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv" // .env dosyasını os.Getenv'e yükler
)

// Config, uygulamanın tüm yapılandırma değerlerini bir arada tutar.
// Struct field'ları export edilmiş (büyük harf) — diğer paketlerden erişilebilir.
type Config struct {
	// Veritabanı
	DatabaseURL string // URL formatında bağlantı cümlesi (Railway vb. için öncelikli)
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// Google Drive
	DriveFolderID string // Taranacak klasörün Drive ID'si
	GoogleAPIKey  string // Google API Key (herkese açık klasörler için)

	// Güvenlik (API anahtarı — her istekte X-API-Key header'ı)
	APIKey string

	// Sunucu
	Port    string // Örn: "8080"
	GinMode string // "debug" veya "release"
}

// Load, .env dosyasını yükler ve Config struct'ını doldurarak döner.
// Eksik kritik değerler varsa hata döner; uygulama başlamaz.
func Load() (*Config, error) {
	// .env dosyasını oku (dosya yoksa hata vermez; environment değişkenleri yeterli)
	// net/http karşılığı yok — bu sadece bir yardımcı kütüphane
	_ = godotenv.Load()

	cfg := &Config{
		DatabaseURL:   getEnv("DATABASE_URL", ""),
		// Fallback zinciri: DB_HOST -> PGHOST (Railway) -> localhost
		DBHost:        getEnv("DB_HOST", getEnv("PGHOST", "localhost")),
		DBPort:        getEnv("DB_PORT", getEnv("PGPORT", "5433")), // local default'u 5433 bıraktık
		DBUser:        getEnv("DB_USER", getEnv("PGUSER", "postgres")),
		DBPassword:    getEnv("DB_PASSWORD", getEnv("PGPASSWORD", "")),
		DBName:        getEnv("DB_NAME", getEnv("PGDATABASE", "art_dataset")),
		DriveFolderID: getEnv("GOOGLE_DRIVE_FOLDER_ID", ""),
		GoogleAPIKey:  getEnv("GOOGLE_API_KEY", ""),
		APIKey:        getEnv("API_KEY", ""),
		Port:          getEnv("PORT", "8080"),
		GinMode:       getEnv("GIN_MODE", "debug"),
	}

	// Kritik değerlerin kontrolü — eksikse uygulamayı başlatma
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API_KEY ortam değişkeni boş olamaz")
	}
	if cfg.DatabaseURL == "" && cfg.DBPassword == "" {
		return nil, fmt.Errorf("DB_PASSWORD veya DATABASE_URL ortam değişkeni tanımlı olmalıdır")
	}
	// GOOGLE_API_KEY olmadan sync scripti çalışmaz; API sunucusu çalışmaya devam eder
	if cfg.GoogleAPIKey == "" {
		// Sadece uyarı — API Key olmadan Drive sync yapılamaz ama sunucu çalışır
		_ = fmt.Sprintf("Uyarı: GOOGLE_API_KEY tanımlı değil, Drive sync çalışmaz")
	}

	return cfg, nil
}

// DSN, pgx/pgxpool için bağlantı dizesini oluşturur.
// Örn: "host=localhost port=5432 user=postgres password=... dbname=art_dataset sslmode=disable"
func (c *Config) DSN() string {
	// Eğer ortamda DATABASE_URL hazır verilmişse onu kullan (örn: Railway)
	if c.DatabaseURL != "" {
		return c.DatabaseURL
	}

	return fmt.Sprintf(
		// sslmode=prefer: Railway PostgreSQL hem SSL hem plain bağlantıyı destekler
		// sslmode=disable bazı cloud ortamlarında bağlantıyı reddeder
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=prefer",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName,
	)
}

// getEnv, ortam değişkenini okur; yoksa varsayılan değeri döner.
// net/http projelerinde de aynı şekilde kullanılır.
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
