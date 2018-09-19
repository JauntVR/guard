FROM golang:1.10.1-alpine as builder
ENV GUARD_SOURCE_PATH $GOPATH/src/github.com/appscode/guard
WORKDIR ${GUARD_SOURCE_PATH}
ADD . ${GUARD_SOURCE_PATH}
RUN apk add --update --no-cache ca-certificates \
    && apk add python py-pip git bash \
    && pip install -r requirements.txt \
    && go get golang.org/x/tools/cmd/goimports \
    && ./hack/builddeps.sh
RUN ./hack/make.py

FROM alpine

RUN set -x \
  && apk add --update --no-cache ca-certificates

COPY --from=builder /go/bin/guard /usr/bin/guard

ENTRYPOINT ["guard"]
