FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server && \
    CGO_ENABLED=0 go build -o /out/worker ./cmd/worker

FROM alpine:3.22
RUN addgroup -S app && adduser -S app -G app && apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /out/worker /app/worker
USER app
EXPOSE 8080
ENTRYPOINT ["/app/server"]

