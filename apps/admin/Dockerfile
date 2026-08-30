FROM golang:1.26.5-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/teamgg-admin .

FROM alpine:3.22 AS runtime

RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S teamgg \
    && adduser -S -G teamgg teamgg
WORKDIR /app
COPY --from=build /out/teamgg-admin /app/teamgg-admin
USER teamgg
EXPOSE 7730
ENTRYPOINT ["/app/teamgg-admin"]
