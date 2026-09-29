FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /resume ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates fonts-dejavu-core && rm -rf /var/lib/apt/lists/*
RUN useradd --system --uid 10001 --create-home resume
WORKDIR /app
COPY --from=build /resume /usr/local/bin/resume
RUN mkdir -p /app/data /app/uploads && chown -R resume:resume /app
USER resume
ENV PORT=8080 DATABASE_PATH=/app/data/resume.db UPLOAD_PATH=/app/uploads FONT_PATH=/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf
EXPOSE 8080
ENTRYPOINT ["resume"]
