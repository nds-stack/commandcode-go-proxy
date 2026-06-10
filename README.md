# CommandCode Go Proxy

<p align="center">
  <img src="assets/banner.jpg" alt="CommandCode Go Proxy" width="100%">
</p>

Proxy server kompatibel OpenAI untuk pengguna CommandCode plan **Go**.

Repository: https://github.com/nds-stack/commandcode-go-proxy  
Versi: `v0.1.0`

## Apa ini?

CommandCode punya 4 plan: **Go**, **Pro**, **Max**, **Ultra**.

Yang jadi masalah — plan Pro, Max, dan Ultra bisa langsung akses API CommandCode lewat endpoint OpenAI/Anthropic. Tapi plan **Go?** Tidak bisa. Bahkan sekadar hit endpoint OpenAI kompatibel pun tidak diijinkan.

Proxy ini solusinya. Jalankan proxy lokal ini, dan kamu bisa pakai CommandCode plan Go seperti biasa lewat endpoint OpenAI kompatibel.

## Install & Jalankan

```bash
git clone https://github.com/nds-stack/commandcode-go-proxy.git
cd commandcode-go-proxy
go run main.go
```

Server berjalan di `http://127.0.0.1:9173`.

Mau port/host lain?

```bash
go run main.go -port 3000 -host 0.0.0.0
```

Mau simpan API key biar tidak perlu kirim di setiap request?

```bash
go run main.go -api-key commandcode-api-key-kamu
```

## Pakai

Proxy ini expose endpoint yang 100% kompatibel sama OpenAI API. Jadi kamu bisa pakai client OpenAI library apapun, ganti base URL ke proxy ini, dan langsung jalan.

### Contoh dengan curl

```bash
curl http://127.0.0.1:9173/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer commandcode-api-key-kamu" \
  -d '{
    "model": "deepseek-v4-pro",
    "messages": [{"role": "user", "content": "Halo!"}]
  }'
```

### Contoh dengan OpenAI Python SDK

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://127.0.0.1:9173/v1",
    api_key="commandcode-api-key-kamu"
)

response = client.chat.completions.create(
    model="deepseek-v4-pro",
    messages=[{"role": "user", "content": "Halo!"}]
)
print(response.choices[0].message.content)
```

### Streaming

```python
stream = client.chat.completions.create(
    model="deepseek-v4-pro",
    messages=[{"role": "user", "content": "Tulis puisi pendek."}],
    stream=True
)
for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

## Endpoint

| Method | Path | Fungsi |
|--------|------|--------|
| `POST` | `/v1/chat/completions` | Chat completions (kompatibel OpenAI) |
| `POST` | `/chat/completions` | Alias yang sama |
| `POST` | `/v1/responses` | Responses API (kompatibel OpenAI) |
| `GET` | `/v1/models` | Daftar model |
| `GET` | `/health` | Health check |

## Model

Proxy menerima full model ID dan alias pendek:

| Alias | Model |
|-------|-------|
| `deepseek-v4-pro`, `deepseek-v4`, `deepseek-pro` | `deepseek/deepseek-v4-pro` |
| `deepseek-v4-flash`, `deepseek-flash` | `deepseek/deepseek-v4-flash` |
| `minimax-m2.7`, `minimax2.7` | `MiniMaxAI/MiniMax-M2.7` |
| `minimax-m2.5`, `minimax2.5`, `minimax` | `MiniMaxAI/MiniMax-M2.5` |
| `glm-5.1` | `zai-org/GLM-5.1` |
| `glm-5` | `zai-org/GLM-5` |
| `kimi-k2.6`, `kimi2.6` | `moonshotai/Kimi-K2.6` |
| `kimi-k2.5`, `kimi2.5` | `moonshotai/Kimi-K2.5` |
| `qwen-3.6-max-preview`, `qwen3.6-max` | `Qwen/Qwen3.6-Max-Preview` |
| `qwen-3.6-plus`, `qwen3.6-plus`, `qwen3.6` | `Qwen/Qwen3.6-Plus` |
| `step-3.5-flash`, `step3.5` | `stepfun/Step-3.5-Flash` |
| `gemini-3.1-flash-lite`, `gemini-flash-lite` | `google/gemini-3.1-flash-lite` |

Nama model yang tidak dikenal diteruskan apa adanya ke CommandCode.

## API Key

Proxy cek API key dengan urutan:

1. Header `Authorization: Bearer <key>` dari request
2. Flag `-api-key` saat menjalankan proxy
3. Kalau keduanya kosong → `401 Unauthorized`

## Build

```bash
# Platform saat ini
go build -o bin/commandcode-go-proxy

# Windows
GOOS=windows GOARCH=amd64 go build -o bin/commandcode-go-proxy.exe

# Linux
GOOS=linux GOARCH=amd64 go build -o bin/commandcode-go-proxy
```

## Opsi Lengkap

| Flag | Default | Keterangan |
|------|---------|------------|
| `-host` | `127.0.0.1` | Host binding |
| `-port` | `9173` | Port server |
| `-api-key` | - | API key default |
| `-timeout` | `600s` | Timeout request |
| `-debug` | `false` | Debug logging |
| `-version` | `false` | Cek versi |

## Cara Kerja

```
Client (OpenAI SDK/curl)
        │
        ▼
┌─────────────────────┐
│  CommandCode Go     │  localhost:9173
│  Proxy              │
└────────┬────────────┘
         │ konversi format OpenAI → CommandCode
         ▼
┌─────────────────────┐
│  api.commandcode.ai │  /alpha/generate
└─────────────────────┘
```

1. Client kirim request format OpenAI
2. Proxy konversi ke format CommandCode (extract system message, map model, convert messages)
3. Proxy forward ke `api.commandcode.ai/alpha/generate`
4. Respons streaming NDJSON dari CommandCode dikonversi balik ke SSE format OpenAI

## Struktur Project

```
├── main.go                    # Entry point, CLI flags, startup info
├── assets/
│   └── banner.jpg             # Banner
├── internal/
│   ├── api/                   # Tipe data CommandCode & OpenAI
│   │   ├── commandcode.go     # Request/response CommandCode
│   │   └── openai.go          # Request/response OpenAI
│   ├── proxy/                 # Logika proxy utama
│   │   ├── proxy.go           # Handler, streaming, retry
│   │   ├── convert.go         # Konversi message & tool format
│   │   └── model.go           # Pemetaan nama model
│   ├── server/                # HTTP server wrapper
│   │   └── server.go
│   ├── update/                # Cek versi terbaru dari GitHub
│   │   └── update.go
│   └── version/               # Fetch versi command-code dari npm
│       └── version.go
└── test/                      # Script testing
```

## License

MIT
