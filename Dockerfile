FROM golang:1.8.3 as builder
WORKDIR $GOPATH/src/github.com/jauntvr/guard
COPY . $GOPATH/src/github.com/jauntvr/guard
RUN ./hack/builddeps.sh && ./hack/make.py build
RUN chmod 755 $GOPATH/src/github.com/jauntvr/guard/dist/guard/guard-alpine-amd64

FROM alpine

RUN set -x \
  && apk add --update --no-cache ca-certificates

COPY --from=builder $GOPATH/src/github.com/jauntvr/guard/dist/guard/guard-alpine-amd64 /usr/bin/guard

USER nobody:nobody
ENTRYPOINT ["guard"]
