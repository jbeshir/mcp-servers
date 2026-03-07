FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/amazon-products-mcp ./cmd/amazon-products-mcp

FROM chromedp/headless-shell:latest
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /app/amazon-products-mcp /app/amazon-products-mcp
ENTRYPOINT ["/app/amazon-products-mcp"]
