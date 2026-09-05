# Образ Go-сервера с собранным frontend. Сборка всегда из корня репозитория:
#   docker build -f Dockerfile .
# frontend/dist копируется в /app/public; до готовности frontend используется
# явно помеченная минимальная страница (это не утверждённый интерфейс).

# --- frontend ---
FROM node:22-alpine AS frontend
WORKDIR /repo
COPY . .
# Собираем frontend, если он уже доставлен; иначе кладём помеченную
# минимальную страницу, чтобы образ оставался собираемым.
RUN if [ -f frontend/package.json ]; then \
      cd frontend && npm ci && npm run build && mkdir -p /out && cp -r dist/. /out/; \
    else \
      mkdir -p /out && printf '<!doctype html><html lang="en"><head><meta charset="utf-8"><title>cosmohack</title></head><body><p>Minimal placeholder page: frontend is not implemented yet.</p></body></html>' > /out/index.html; \
    fi

# --- go build ---
FROM golang:1.27-alpine AS build
WORKDIR /repo/backend
COPY backend/go.mod backend/go.sum* ./
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 go build -trimpath -o /out/server ./cmd/server

# --- runtime: scratch — статический бинарник, CA-сертификаты и статика ---
# CGO_ENABLED=0 даёт статический бинарник; шелла и пакетного менеджера нет.
# CA-сертификаты нужны HTTPS-вызовам наружу (спутники/погода, зона B1).
# Non-root без adduser: числовой uid, совпадающий с владельцем тома в деплое.
FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/server /app/server
COPY --from=frontend /out /app/public
ENV HTTP_ADDR=:8080
ENV PUBLIC_DIR=/app/public
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/app/server"]
