ARG VERSION=0.0.0-dev

FROM golang:1.25 AS builder

ARG VERSION

WORKDIR /app

COPY go.mod go.sum ./
ENV GOTOOLCHAIN=local
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-X main.version=${VERSION}" -o /demo-app ./cmd/server

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /demo-app /demo-app

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/demo-app"]