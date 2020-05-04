* TBD

[![aku](https://circleci.com/gh/aku/geofilter.svg?style=svg)](https://circleci.com/gh/aku/geofilter)
[![Go Report Card](https://goreportcard.com/badge/github.com/aku/geofilter)](https://goreportcard.com/report/github.com/aku/geofilter)
![Docker Image Version (tag latest semver)](https://img.shields.io/docker/v/akuritsyn/geofilter/latest?label=Docker%20Hub)

docker run -ti --rm --network host -v geo.mmdb:/db.mmdb geofilter -p 6000 -d ./db.mmdb -a US -t http://localhost:4000
