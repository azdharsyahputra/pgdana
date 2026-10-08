# ⚡ QRIS Gateway & Mutasi Scraper Engine (Full Golang)

Gateway pembayaran QRIS dinamis mandiri berbasis **Golang murni**. Dirancang dengan arsitektur **Zero-Scraping / Anti-Banned**, performa tinggi, konkurensi aman (*thread-safe*), dan anti klaim ganda (*anti double-claim*) menggunakan sistem **3-digit kode unik**.

---

## 🌟 Keunggulan Dibanding Gateway Unofficial Biasa

| Masalah pada Gateway Biasa | Solusi pada Project Golang Ini |
| :--- | :--- |
| **Akun Merchant Sering Diblokir Gojek/Bank** karena scraping API private dari IP VPS. | **100% Zero-Scraping**: Memanfaatkan listener notifikasi Android (MacroDroid/Tasker) atau webhook mutasi resmi. Akun 100% aman dari deteksi bot! |
| **Double-Claim Concurrency**: Dua orang bayar nominal sama di menit yang sama, pembayaran tertukar. | **Kode Unik Otomatis (1-999)**: Setiap invoice memiliki nominal akhir unik (cth: Rp 50.142) sehingga verifikasi instan & mustahil tertukar. |
| **In-Memory Data Loss**: Server reboot/crash, data invoice hilang. | **Persistent Storage**: Data tersimpan otomatis ke disk (`data/invoices.json`). Aman saat restart. |
| **Performa & Ukuran File**: Node.js butuh `node_modules` ratusan MB & RAM besar. | **Single Native Binary**: Kompilasi murni Go (~15 MB), konsumsi RAM sangat hemat (< 20 MB), startup instan (< 0.05 detik). |

---

## 📁 Struktur Direktori

```
qris-gateway-go/
├── cmd/
│   └── server/
│       └── main.go          # Entrypoint server & router (Go 1.22+ standard mux)
├── pkg/
│   ├── config/              # Loader konfigurasi environment & .env
│   ├── qris/                # EMVCo TLV Parser, CRC16 Checksum, QR Code Generator
│   ├── invoice/             # Invoice store, thread-safe memory + disk persistence
│   ├── webhook/             # Parser notifikasi (GoBiz, BCA, Mandiri, DANA, Shopee) & dispatcher
│   └── handler/             # Handler REST API & Web Checkout UI
├── web/
│   └── template.html        # Halaman checkout web responsif + polling real-time
├── data/                    # Direktori penyimpanan invoice persistent (auto-created)
├── .env.example             # Template variabel environment
├── Makefile                 # Shortcut build & run
└── README.md
```

---

## 🚀 Cara Menjalankan

### 1. Prasyarat
Pastikan Go versi 1.22 atau lebih baru sudah terpasang:
```bash
go version
```

### 2. Konfigurasi `.env`
Salin template konfigurasi:
```bash
cp .env.example .env
```
Sesuaikan isi `.env`:
```env
PORT=8080
API_KEY=my-secret-api-key
WEBHOOK_SECRET=my-webhook-secret-token
MERCHANT_NAME=Toko Digital Saya

# Masukkan kode QRIS Statis Anda (GoPay / BCA / Mandiri / Shopee / dll)
QRIS_STATIC=000201010211...

# Aktifkan 3-digit kode unik agar verifikasi 100% otomatis tanpa tertukar
USE_UNIQUE_CODE=true

# Webhook callback ke sistem toko / bot Anda saat pembayaran lunas (opsional)
CLIENT_WEBHOOK_URL=https://toko-kamu.com/api/payment-callback
```

### 3. Build & Jalankan
Gunakan `Makefile`:
```bash
# Jalankan langsung
make run

# Atau jalankan test
make test
```
Server akan aktif di: `http://localhost:8080`

---

## 📱 Setup Otomasi Notifikasi Android (MacroDroid / Tasker)

Metode ini membuat sistem **bebas risiko banned selamanya** karena server Anda tidak melakukan scraping ke API Gojek/Bank.

### Langkah-langkah di MacroDroid (Gratis di Play Store):
1. **Trigger**:
   - Pilih `Notification` -> `Notification Received`.
   - Pilih aplikasi: **GoBiz** / **BCA mobile** / **Livin by Mandiri** / **DANA** / **ShopeePay**.
2. **Action**:
   - Pilih `Web Interaction` -> `HTTP Request`.
   - **URL**: `http://IP-VPS-KAMU:8080/api/webhook/notification`
   - **Method**: `POST`
   - **Header**:
     - `Content-Type`: `application/json`
     - `X-Webhook-Secret`: `my-webhook-secret-token` *(sesuai .env)*
   - **Request Body**:
     ```json
     {
       "text": "[notification]"
     }
     ```
     *(Tips: Bisa juga klik tombol Magic Text `[...]` di samping kolom ➡️ pilih **Notification** / **Pemberitahuan** ➡️ pilih **Notification Text** (`[notification]`)).*
3. **Simpan Macro**. Setiap ada transaksi masuk di HP, notifikasi otomatis diteruskan ke gateway dalam hitungan milidetik dan invoice langsung lunas!

---

## 📚 Dokumentasi REST API

### 1. Buat Invoice QRIS Dinamis
`POST /api/qris/create` (atau via `GET` query param)

**Headers:**
`X-API-Key: my-secret-api-key`

**Body:**
```json
{
  "amount": 50000,
  "order_id": "ORDER-1001",
  "customer_info": "User A"
}
```

**Respon Sukses:**
```json
{
  "success": true,
  "message": "QRIS dinamis berhasil dibuat",
  "checkout_url": "http://localhost:8080/pay/INV-261008-7F8CBD32",
  "invoice": {
    "id": "INV-261008-7F8CBD32",
    "order_id": "ORDER-1001",
    "base_amount": 50000,
    "unique_code": 142,
    "total_amount": 50142,
    "qris_content": "000201010212...",
    "qris_base64": "data:image/png;base64,...",
    "status": "PENDING",
    "expires_at": "2026-10-08T14:32:13Z"
  }
}
```

---

### 2. Cek Status Pembayaran
`GET /api/qris/status/{id}`

**Respon:**
```json
{
  "success": true,
  "paid": true,
  "invoice": {
    "id": "INV-261008-7F8CBD32",
    "status": "PAID",
    "paid_at": "2026-10-08T14:18:01Z",
    "payment_data": {
      "source": "GOPAY",
      "raw_message": "GoBiz: Pembayaran QRIS Rp 50.142 berhasil diterima"
    }
  }
}
```

---

### 3. Tampilkan Gambar QR Code Langsung
`GET /api/qris/qr/{id}`

Mengembalikan gambar mentah format `image/png` yang dapat langsung di-embed di tag `<img src="...">` aplikasi Anda.

---

### 4. Halaman Checkout Pembeli
`GET /pay/{id}`

Membuka antarmuka web modern dengan fitur:
- Tampilan QR Code resmi siap scan.
- Tombol salin nominal persis (termasuk kode unik).
- Timer hitung mundur 15 menit.
- **Auto-polling setiap 3 detik**: Begitu dana masuk, layar otomatis berganti ke animasi sukses pembayaran lunas tanpa perlu refresh browser.

---

### 5. Webhook Notifikasi Mutasi Masuk
`POST /api/webhook/notification`

**Headers:**
`X-Webhook-Secret: my-webhook-secret-token`

**Body:**
```json
{
  "text": "GoBiz: Pembayaran QRIS Rp 50.142 berhasil diterima"
}
```
*Didukung format notifikasi dari GoBiz, BCA, Mandiri Livin, DANA Bisnis, ShopeePay Merchant, OVO, dan LinkAja.*

---

## 🛠️ Deploy ke VPS Menggunakan Systemd

Agar gateway berjalan terus di latar belakang (background) dan otomatis restart jika VPS reboot:

1. Build binary di VPS:
   ```bash
   go build -o /opt/qris-gateway/server ./cmd/server
   ```
2. Buat service systemd:
   ```ini
   # /etc/systemd/system/qris-gateway.service
   [Unit]
   Description=QRIS Gateway Golang
   After=network.target

   [Service]
   Type=simple
   User=root
   WorkingDirectory=/opt/qris-gateway
   ExecStart=/opt/qris-gateway/server
   Restart=always
   RestartSec=5

   [Install]
   WantedBy=multi-user.target
   ```
3. Aktifkan service:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now qris-gateway
   ```
