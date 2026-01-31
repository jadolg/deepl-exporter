FROM golang:1.25-alpine AS builder
RUN apk --no-cache add ca-certificates
WORKDIR /app

COPY go.mod go.sum /app/
RUN go mod download

COPY main.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o deepl-exporter .

FROM scratch

COPY --from=builder /app/deepl-exporter /deepl-exporter
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 1818
ENV PORT=1818

CMD ["/deepl-exporter"]
