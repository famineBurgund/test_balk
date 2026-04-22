FROM golang:1.25 AS build

WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o /out/disloc-service ./cmd/app

FROM gcr.io/distroless/base-debian12

WORKDIR /app
COPY --from=build /out/disloc-service /app/disloc-service

EXPOSE 50070
ENTRYPOINT ["/app/disloc-service"]
