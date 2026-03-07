FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/manifold-mcp ./cmd/manifold-mcp

FROM gcr.io/distroless/static-debian12
COPY --from=build /app/manifold-mcp /app/manifold-mcp
ENTRYPOINT ["/app/manifold-mcp"]
