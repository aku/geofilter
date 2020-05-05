FROM golang:1.14 AS build-env

WORKDIR /src
COPY . /src
RUN CGO_ENABLED=0 GOOS=linux \
    go build \
    -a -installsuffix cgo \
    -o geofilter \
    .

FROM gruebel/upx:latest as upx

COPY --from=build-env /src/geofilter /geofilter_orig
RUN upx --best --lzma -qqq -o /geofilter_compressed /geofilter_orig

FROM scratch

LABEL maintainer="Alexander Kuritsyn" \
      org.opencontainers.image.authors="Alexander Kuritsyn" \
      org.opencontainers.image.title="geofilter" \
      org.opencontainers.image.description="GeoIP reverse proxy"

COPY --from=upx /geofilter_compressed /geofilter

EXPOSE 80

ENTRYPOINT ["./geofilter"]
