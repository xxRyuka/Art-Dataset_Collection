// Package middleware — HTTP isteklerine uygulanacak ara katmanlar.
// Gin'de middleware, handler çalışmadan önce (veya sonra) devreye girer.
//
// net/http karşılığı:
//   Gin:     router.Use(MyMiddleware())
//   net/http: handler = MyMiddleware(handler)  (handler wrapping)

package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin" // net/http karşılığı: "net/http"
)

// APIKeyAuth, X-API-Key header'ını kontrol eden middleware'dir.
// Geçersiz veya eksik key varsa isteği 401 ile reddeder ve durdurar.
//
// Gin middleware ile net/http middleware karşılaştırması:
//
//   GIN:
//     func APIKeyAuth() gin.HandlerFunc {
//         return func(c *gin.Context) { ... c.Next() }
//     }
//
//   net/http:
//     func APIKeyAuth(next http.Handler) http.Handler {
//         return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//             ... next.ServeHTTP(w, r)
//         })
//     }
//
// İkisi de aynı mantıkla çalışır; Gin sadece c.Context ile w ve r'yi birleştirir.
func APIKeyAuth() gin.HandlerFunc {
	// gin.HandlerFunc = func(c *gin.Context)
	// net/http'de bu: http.HandlerFunc = func(w http.ResponseWriter, r *http.Request)
	return func(c *gin.Context) {
		// Header'dan API key'i al
		// net/http karşılığı: r.Header.Get("X-API-Key")
		key := c.GetHeader("X-API-Key")

		expectedKey := os.Getenv("API_KEY")

		if key == "" || key != expectedKey {
			// İsteği reddet ve durdur
			// net/http karşılığı:
			//   w.WriteHeader(http.StatusUnauthorized)
			//   json.NewEncoder(w).Encode(map[string]string{"error": "..."})
			//   return
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				// gin.H = map[string]any — JSON yanıt için kısaltma
				"error": "geçersiz veya eksik API anahtarı",
				"hint":  "X-API-Key header'ını ekleyin",
			})
			// c.Abort(): sonraki handler'ların çalışmasını durdurur
			// net/http karşılığı: return (chain'i manuel kesmek)
			return
		}

		// Geçerli key — bir sonraki handler'a geç
		// net/http karşılığı: next.ServeHTTP(w, r)
		c.Next()
	}
}

// CORS, Cross-Origin Resource Sharing başlıklarını ayarlar.
// Frontend (tarayıcıdan) API'yi çağıracaksa gereklidir.
// Örn: localhost:3000'deki frontend, localhost:8080'deki API'yi çağırır.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		// net/http karşılığı: w.Header().Set("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, X-API-Key")

		// OPTIONS isteği (preflight) — tarayıcı önce bunu gönderir
		if c.Request.Method == "OPTIONS" {
			// net/http karşılığı: w.WriteHeader(http.StatusNoContent); return
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
