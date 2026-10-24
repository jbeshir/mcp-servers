FROM golang:1.27.3 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/assets-mcp ./cmd/assets-mcp

FROM gcr.io/distroless/static-debian13
COPY --from=build /app/assets-mcp /app/assets-mcp
ENTRYPOINT ["/app/assets-mcp"]
