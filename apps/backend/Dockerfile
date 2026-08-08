# syntax=docker/dockerfile:1

FROM golang:1.26.5-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/teamgg .

FROM debian:bookworm-slim AS runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --uid 10001 teamgg

WORKDIR /app
COPY --from=builder /out/teamgg ./teamgg
RUN mkdir -p /app/datafiles \
    && chown -R teamgg:teamgg /app

USER teamgg
ENV APP_SERVER_PORT=7713 \
    APP_PROJECT_ROOT=/app

VOLUME ["/app/datafiles"]
EXPOSE 7713

ENTRYPOINT ["./teamgg"]
