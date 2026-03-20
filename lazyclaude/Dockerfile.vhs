FROM ghcr.io/charmbracelet/vhs:latest

RUN apt-get update && apt-get install -y --no-install-recommends \
    tmux golang-go \
    && rm -rf /var/lib/apt/lists/*

ENV TERM=xterm-256color

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /usr/local/bin/lazyclaude ./cmd/lazyclaude

ENTRYPOINT ["vhs"]
