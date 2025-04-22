# ---- Build Stage ----
FROM golang:1.24.1-alpine AS builder

WORKDIR /app

COPY go.mod ./

RUN go mod download

COPY *.go ./

RUN go test

# Compile a aplicação Go.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server .

# ---- Run Stage ----
# Use a imagem distroless/static que é mínima, segura e inclui certificados CA
FROM gcr.io/distroless/static-debian11

WORKDIR /app

# Copie APENAS o binário compilado da etapa de build
COPY --from=builder /app/server /app/server

# NENHUM RUN ou EXPOSE é estritamente necessário aqui para Distroless,
# mas EXPOSE documenta a porta. O usuário do container é não-root por padrão.
EXPOSE 8080

# Comando para rodar a aplicação (o entrypoint padrão da distroless/static funciona,
# mas especificar o CMD é mais explícito)
CMD ["/app/server"]