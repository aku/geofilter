* TBD

[![aku](https://circleci.com/gh/aku/geofilter.svg?style=svg)](https://circleci.com/gh/aku/geofilter)

[https://hub.docker.com/r/akuritsyn/geofilter](https://hub.docker.com/r/akuritsyn/geofilter)

docker run -ti --rm --network host -v geo.mmdb:/db.mmdb geofilter -p 6000 -d ./db.mmdb -a US -t http://localhost:4000