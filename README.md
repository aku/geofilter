[![aku](https://circleci.com/gh/aku/geofilter.svg?style=svg)](https://circleci.com/gh/aku/geofilter)
[![Go Report Card](https://goreportcard.com/badge/github.com/aku/geofilter)](https://goreportcard.com/report/github.com/aku/geofilter)
![Docker Image Version (tag latest semver)](https://img.shields.io/docker/v/akuritsyn/geofilter/latest?label=Docker%20Hub)

A proxy server that blocks/allows requests based on a client's country.
It uses [MaxMind GeoIP2](https://www.maxmind.com/en/geoip2-databases) geolocation database to get a client's country.


docker run -ti --rm --network host -v geo.mmdb:/db.mmdb geofilter -p 6000 -d ./db.mmdb -a US -t http://localhost:4000
