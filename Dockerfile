# ── Aşama 1: Builder ────────────────────────────────────────────
# Go uygulamasını derlemek için resmi Go imajı kullanılır
# Bu aşama sadece derleme için; son imaja dahil olmaz (çok küçük imaj elde ederiz)
FROM golang:1.23-alpine AS builder

# Gerekli derleme araçları (Alpine'de eksik olabilir)
RUN apk add --no-cache git

WORKDIR /app

# Bağımlılıkları önce kopyala (katman önbellekleme — kod değişmeden bağımlılıklar yeniden indirilmez)
COPY go.mod go.sum ./
RUN go mod download

# Kaynak kodu kopyala
COPY . .

# Uygulamayı statik olarak derle
# CGO_ENABLED=0: Saf Go binary, C kütüphanesi bağımlılığı yok
# -ldflags="-w -s": Debug sembollerini çıkar → daha küçük binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /art-api ./cmd/api

# ── Aşama 2: Runtime ────────────────────────────────────────────
# Çalışma zamanı için scratch (boş) veya distroless kullanılabilir
# alpine: Hata ayıklamak için shell erişimi istiyorsan daha iyi
FROM alpine:3.20

# Zaman dilimi ve SSL sertifikaları (HTTP istekleri için gerekli)
RUN apk add --no-cache ca-certificates tzdata

# Türkiye zaman dilimini ayarla
ENV TZ=Europe/Istanbul

WORKDIR /app

# Sadece derlenmiş binary'yi kopyala (kaynak kod imajda yok)
COPY --from=builder /art-api .

# Uygulamanın çalıştığı port (docker-compose.yml ile eşleşmeli)
EXPOSE 8080

# Container başladığında çalıştırılacak komut
ENTRYPOINT ["./art-api"]
