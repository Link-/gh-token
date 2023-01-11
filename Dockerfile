FROM golang:1.18-buster as builder

WORKDIR /src
COPY . .
RUN go build ./...

# Use multi-stage images to cleanup build artifacts.
FROM debian:11.6 as final

WORKDIR /app

COPY --from=builder /src/gh-token /app/gh-token 

ENV PATH "$PATH:/app"

CMD ["/app/ghtoken"]
