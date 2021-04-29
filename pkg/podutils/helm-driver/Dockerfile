FROM golang:1.15 as build-env

WORKDIR /go/src/helm
ADD . /go/src/helm

RUN go get -d -v ./...

RUN go build -o /go/bin/helm-driver

FROM gcr.io/distroless/base
COPY --from=build-env /go/bin/helm-driver /