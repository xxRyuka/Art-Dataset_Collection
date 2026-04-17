// Package drive — Google Drive API v3 istemcisi.
//
// Klasör herkese açık (public) olduğu için Service Account yerine
// basit Google API Key kullanıyoruz.
//
// Service Account   → Özel klasörler için, credentials.json gerekir (karmaşık)
// Google API Key    → Herkese açık klasörler için, tek bir string yeterli (bizim durumumuz)
//
// API Key nasıl alınır?
//   1. console.cloud.google.com → Proje oluştur (ücretsiz)
//   2. APIs & Services → Enable APIs → "Google Drive API" aç
//   3. APIs & Services → Credentials → "Create Credentials" → "API Key"
//   4. API Key'i kopyala → .env'e GOOGLE_API_KEY olarak yapıştır
//
// Klasörün "herkese açık" olması ne anlama gelir?
//   Drive'da klasörü sağ tıkla → Share → "Anyone with the link" → Viewer
//   Bu ayarla, Drive API'si API Key ile dosyaları listeleyebilir.
//   credentials.json, service account, paylaşım e-postası HİÇBİRİNE gerek yok.

package drive

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/drive/v3" // Drive API v3
	"google.golang.org/api/option"   // Client seçenekleri
)

// Client, Google Drive API'si için sarmalayıcı struct.
// Tüm Drive işlemleri bu struct üzerinden yapılır.
type Client struct {
	service  *drive.Service // Google Drive API servisi
	folderID string         // Taranacak klasörün Drive ID'si
}

// NewClient, Google API Key ile Drive API istemcisi oluşturur.
// Herkese açık klasörler için Service Account yerine API Key yeterlidir.
//
// apiKey  : .env'deki GOOGLE_API_KEY değeri
// folderID: .env'deki GOOGLE_DRIVE_FOLDER_ID değeri
func NewClient(ctx context.Context, apiKey, folderID string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf(
			"GOOGLE_API_KEY boş olamaz\n" +
				"Almak için: console.cloud.google.com → APIs & Services → Credentials → API Key",
		)
	}

	// option.WithAPIKey: Kimlik doğrulama için sadece bir string gerekir
	// Service Account: JSON dosyası, IAM ayarları, paylaşım gerekir (karmaşık)
	// API Key:         Tek string, herkese açık içerik için yeterli (basit)
	svc, err := drive.NewService(ctx,
		option.WithAPIKey(apiKey),
	)
	if err != nil {
		return nil, fmt.Errorf("Drive servisi oluşturulamadı: %w", err)
	}

	return &Client{
		service:  svc,
		folderID: folderID,
	}, nil
}

// DriveFile, Drive'dan gelen ham dosya bilgisini temsil eder.
// domain.Image'a dönüştürülmeden önce bu struct kullanılır.
type DriveFile struct {
	ID   string // Drive dosya ID'si
	Name string // Dosya adı
}

// ListAllFiles, belirlenen klasördeki tüm dosyaları sayfalandırmalı olarak getirir.
// Drive API, bir seferde max 1000 dosya döner; pageToken ile sonraki sayfaları alırız.
//
// Toplam 14.000 dosya için yaklaşık 14 sayfa (14 API çağrısı) yapılır.
// Drive API kotası: 1000 istek/100 sn — sayfalama tek bir "list" isteğidir, sorun yok.
func (c *Client) ListAllFiles(ctx context.Context) ([]DriveFile, error) {
	var allFiles []DriveFile
	pageToken := "" // Boş: ilk sayfa

	for {
		// Drive API isteği oluştur
		// Sadece ID ve Name alanlarını iste (bant genişliği tasarrufu)
		call := c.service.Files.List().
			// Bu klasörün altındaki, çöp kutusunda olmayan dosyaları getir
			Q(fmt.Sprintf("'%s' in parents and trashed = false", c.folderID)).
			// Hangi alanları istediğimizi belirt
			Fields("nextPageToken, files(id, name)").
			// Bir seferde maksimum dosya sayısı (Drive API limiti: 1000)
			PageSize(1000).
			// Herkese açık klasörler için gerekli — API Key ile erişimde
			// "includeItemsFromAllDrives" ve "supportsAllDrives" bazen gerekir
			SupportsAllDrives(true).
			IncludeItemsFromAllDrives(true)

		// Sayfalama: önceki sayfadan gelen token varsa ekle
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		// API isteğini gönder
		result, err := call.Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("Drive dosyaları listelenemedi: %w\n"+
				"İpucu: Klasörün 'Anyone with the link' olarak paylaşıldığından emin olun", err)
		}

		// Gelen dosyaları listeye ekle
		for _, f := range result.Files {
			allFiles = append(allFiles, DriveFile{
				ID:   f.Id,
				Name: f.Name,
			})
		}

		// Sonraki sayfa yoksa döngüyü bitir
		if result.NextPageToken == "" {
			break
		}

		// Rate limit koruması: 429 hatasını önlemek için kısa bekleme
		time.Sleep(200 * time.Millisecond)

		pageToken = result.NextPageToken
	}

	return allFiles, nil
}
