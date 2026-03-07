FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/supermarkets-uk-mcp ./cmd/supermarkets-uk-mcp

FROM chromedp/headless-shell:latest
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /app/supermarkets-uk-mcp /app/supermarkets-uk-mcp
ENTRYPOINT ["/app/supermarkets-uk-mcp"]
