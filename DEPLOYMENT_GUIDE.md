# Deployment ve Geliştirme Rehberi

Bu doküman, projeyi geliştirirken ve canlıya alırken (production) izlenmesi gereken doğru Branch stratejisini ve projenin Railway üzerinde arkada nasıl çalıştığını anlatır.

## 1. Branch (Dal) Stratejisi

Test edebilmek ve ana canlı projeyi bozmamak için her zaman (en az) **iki ayrı branch (dal)** ile çalışılmalıdır.

*   `main` (veya `master`): **Burası CANLI (Production) ortamdır.** Sadece çalışan, test edilmiş, sorunsuz kodlar burada durur. Son kullanıcıların gördüğü halidir.
*   `dev` (Geliştirme): **Burası test ve yeni özellik ortamıdır.** Biz yeni bir özellik ekleyeceğimizde veya sorun çözeceğimizde işlemleri önce bu branch üzerinde veya bu branch'ten oluşturulan alt branch'lerde (`feature/buton-duzeltme` gibi) yaparız. Test ettikten sonra `main` branch'ine atarız.

### Günlük Geliştirme Akışı Nasıl Olmalı?

1.  Masaüstünüzde çalışırken git'te `dev` branch'inde olduğunuza emin olun.
    ```bash
    git checkout dev
    git pull origin dev
    ```
2.  Değişikliklerinizi yapın.
3.  Yerel (local) ortamınızda `docker-compose up` ile test edin. (veya daha önce oluşturduğumuz test html'i üzerinden).
4.  Çalışıyorsa kodları Github'a `dev` branch'ine yollayın:
    ```bash
    git add .
    git commit -m "Mobilde resmi tasan sorunu çözüldü"
    git push origin dev
    ```
5.  **Canlıya Çıkmak (Deploy):** `dev` branch'indeki bu özellikleri artık canlıya yansıtmak istiyorsanız, en son `main` branch'ine gelip mevcut yapıyla kodu birleştirmeniz (Merge) gerekir.
    ```bash
    git checkout main
    git merge dev
    git push origin main
    ```

---

## 2. Railway Üzerindeki Yapı ve Canlıya Çıkış (Deployment) Süreci

Sen `git push origin main` dediğin anda Github ve Railway entegre olduğu için Railway bunu otomatik algılar ve build işlemini başlatır. Peki arkada neler dönüyor?

### A. Railway'in Projeyi Algılaması (`railway.toml` ve `Dockerfile`)
Railway'in kodunuzu nasıl çalıştıracağını bilmesi gerekir. Bunun için projemizde `railway.toml` dosyasını kullanıyoruz.
Bu dosyanın içerisindeki konfigürasyonlara göre biz Railway'e *"Kendi algoritmalarını kullanmaya zorlama, sana içeriğinde Go kurulumu falan hepsi yazan `Dockerfile`'ı verdik, oradaki komutlara göre bu projeyi çalıştır"* diyoruz. 

**Dockerfile ne işe yarar?**
İçerisinde adım adım sunucuya kurdurduğumuz yönergeler bütünüdür.
Uygulamayı derler ve ufak bir Linux ortamında hazır paket haline getirip ayağa kaldırır.

### B. Railway Ortamları (Environments)
Madem 2 branch kullanacağız (`main` ve `dev`), Railway'de de **iki ayrı ortam (Environment)** açmalıyız. 
Railway arayüzünüzde uygulamanızın içine girdiğinizde sağ üst taraflardan (veya Environment tabından) "New Environment" diyebilirsiniz. (Adını `Staging` veya `Development` koyabilirsiniz).
*   **Production Environment:** Github'daki `main` branch'ine ayarlanır. Ana oylama sitemizdir, env değişkenleri, DB şifreleri buna aittir. Kendi gerçek Domain adresiniz buraya bağlıdır.
*   **Staging/Dev Environment:** Github'daki `dev` branch'ine bağlanmalıdır. Bu ortamı oluşturduktan sonra projenize bir de yeni "Database" (Postgres) bağlamalısınız. `dev` ortamından atacağınız kodlar bu Environment içerisinde devreye girer. Size sağladığı `...up.railway.app` gibi test amaçlı bir domain'i olur. 

Böylece `dev`'e push'ladığınız bir şeyi önce Staging ortamındaki domainden DB'yi bozmadan risksizce test etmiş olursunuz.

### C. Health Check (Sağlık Kontrolü)
Yeni kodunuzu attıktan sonra sunucu ayağa kalkarken ya çöküp "Service Unavailable" hatası verirse? Kullanıcıları hiç koda çekmememiz gerekir.

Railway yeni container'ı (ortamı) başlattığında, kodlarımız içindeki `/health` URL'sine arka planda test isteği atar (Bunu `railway.toml` dosyasında yazdık). 30 saniye içinde sistem `{"status": "ok"}` şeklinde bir geri dönüş almazsa, kodunuzda veya veritabanı bağlantınızda o an bir sorun olduğunu anlar ve sunucu güncellemeyi İPTAL EDER. **Eski ve çalışan en son sürüme geri döner.** 

---

## 3. Olumsuz Bir Durumda Nelere Müdahale Edilir?

*   **Deploy Hatası (Failed):** Railway'de uygulamanıza girdiğinizde kırmızı bir Failed barı var ise kutucuğa tıklayarak **View Logs (Deploy Logs / App Logs)** bölümüne geçin. Loglarda %90 ihtimalle unutulmuş bir değişken bağlantısı (`Database connection failed`) ya da yaptığınız kodda derleme aşamasında çıkan syntatic bir karakter hatası yazacaktır.
*   **Yeni Bir Şifre / Anahtar Eklendiyse:** Yazılımın içerisinden bir entegrasyon vs. yaptınız ve `.env` dosyanıza `XXX_API_KEY=deneme` gibi bir değer girdiniz. Local ortamınızda bunu okuyacaktır ancak `.env` dosyasını Github'a yollamayacağınız için Railway de bilemeyecektir. Railway üzerinden ilgili Environment'ın **Variables** bölümüne gidip manuel olarak `XXX_API_KEY`, değer kısmına da `deneme` yazmanız zorunludur.
*   **Yeni Tablo Sütunu (Kolon) Eklendiğinde Veritabanı Sorunları:** Az evvel yaptığımız `session_id` eklemesi gibi database'e yeni tablo sütunu ekleyeceğimizde, lokalde Go kodumuza `ALTER TABLE` kodunu `migrations` içine yazıyoruz. Railway deploymentı çalıştırdığında bu `migrations` ayağa kalkarken o sütunu test eder ve yoksa veritabanınıza otomatik yeni kolonu açar. Bunda genelde sorun yaşanmaz ama büyük veritabanı değişikliklerinde veritabanına Railway üzerinden direk Database Variables - Query modundan elle müdahele etmeniz gerekebilir. 

## Özet Akış

1. Yeni özellik eklenecekse önce lokal üzerinden kodunu düzenle. Kök dizine oluşturduğum `test-layout.html` dosyasını chrome üzerinden açıp lokal olarak görselliği test et.
2. İşin kod kısmı bitince `git checkout dev`, sonrasında `git add .`, ardından da `git commit -m "aciklama"` yaparak değişiklikleri hazırla. 
3. `git push origin dev` yaparak Railway'dek geliştirme ortamını (Staging) güncelleyip orada mobil - desktop şeklinde yayından sına. İşleyiş doğru ise..
4. `git checkout main`, sonrasında `git merge dev` ve son olarak `git push origin main` yaparak son ve pürüzsüz halini kendi ana `main` canlı projenize yansıtın.
