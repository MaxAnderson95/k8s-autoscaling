FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /demo-app ./cmd/server

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /demo-app /demo-app

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/demo-app"]