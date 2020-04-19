* TBD

docker run -ti --rm --network host -v geo.mmdb:/db.mmdb geofilter -p 6000 -d ./db.mmdb -a US -t http://localhost:4000