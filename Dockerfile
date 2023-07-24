FROM golang as build-env
WORKDIR /go/src/hubitat_exporter
ADD . /go/src/hubitat_exporter
RUN go build -o /go/bin/hubitat_exporter
FROM gcr.io/distroless/base
COPY --from=build-env /go/bin/hubitat_exporter /
CMD ["/hubitat_exporter"]
