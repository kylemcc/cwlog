FROM golang:alpine as builder

ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV CGO_ENABLED 0

RUN set -x \
    apk add --no-cache ca-certificates \
		&& apk add --no-cache --virtual \
		.build-deps \
		bash \
		git \
		gcc \
		make \
		libc-dev \
		libgcc

COPY . /app
WORKDIR /app

RUN make static \
		&& mv cwlog /usr/bin/cwlog \
		&& echo "Build complete."

FROM scratch

COPY --from=builder /usr/bin/cwlog /usr/bin/cwlog
COPY --from=builder /etc/ssl/certs /etc/ssl/certs

ENTRYPOINT ["cwlog"]
