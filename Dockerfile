FROM golang:1.14 AS build-env

WORKDIR /src
ADD . /src
RUN cd /src
RUN CGO_ENABLED=0 GOOS=linux \
    go build \
    -a -installsuffix cgo \
    -o geofilter \
    .

FROM gruebel/upx:latest as upx

COPY --from=build-env /src/geofilter /geofilter_orig
RUN upx --best --lzma -o /geofilter_compressed /geofilter_orig

FROM scratch

COPY --from=upx /geofilter_compressed /geofilter

EXPOSE 80

ENTRYPOINT ["./geofilter"]
