FROM golang:1.18-buster as builder

WORKDIR /src
COPY . .
RUN go build ./...

# Use multi-stage images to cleanup build artifacts.
FROM debian:11.6 as final

WORKDIR /app

COPY --from=builder /src/gh-token /app/ghtoken 

# Least privilege docker user
RUN groupadd --gid 15555 ghtoken \ 
    && useradd --uid 15555 --gid 15555 -ms /bin/false ghtoken\
    && chown -R ghtoken:ghtoken /app
ENV PATH "$PATH:/app"
USER ghtoken

CMD ["/app/ghtoken"]
