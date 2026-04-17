# Build
FROM golang:1.26 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN cd src/ && CGO_ENABLED=0 GOOS=linux go build -o executor

# Run
FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive
WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
	ca-certificates \
	git \
	build-essential \
	pkg-config \
	libcap-dev \
	libsystemd-dev \
	asciidoc \
	xsltproc \
	docbook-xml \
	docbook-xsl \
	uidmap \
	python3.11 \
	&& rm -rf /var/lib/apt/lists/*

RUN git clone https://github.com/ioi/isolate.git /tmp/isolate \
	&& make -C /tmp/isolate \
	&& make -C /tmp/isolate install \
	&& rm -rf /tmp/isolate

RUN useradd -m isolate \
	&& echo "isolate:100000:65536" >> /etc/subuid \
	&& echo "isolate:100000:65536" >> /etc/subgid

COPY --from=builder /app/src/executor /app/executor

CMD ["/app/executor"]
