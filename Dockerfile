FROM golang:alpine as builder

ENV PATH /go/bin:/usr/local/go/bin:/Users/kyle/.jenv/shims:/Users/kyle/.jenv/bin:/usr/local/Cellar/zplug/2.4.2/bin:/usr/local/opt/zplug/bin:/Users/kyle/bin:/Users/kyle/code/go/bin:/usr/local/bin:/usr/local/sbin:/Library/Frameworks/Python.framework/Versions/2.7/bin:/Users/kyle/.cargo/bin:/usr/bin:/bin:/usr/sbin:/sbin:/Library/Apple/usr/bin:/Users/kyle/.jenv/shims:/Users/kyle/.jenv/bin:/usr/local/storm/bin:/usr/local/go/bin:/Users/kyle/goext/bin:/usr/local/mysql/bin:/usr/local/Cellar/go/1.14.2_1/libexec/bin:./node_modules/.bin:/usr/local/Cellar/go/1.14.3/libexec/bin
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
