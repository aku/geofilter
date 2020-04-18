
docker run -ti --rm -p 8080:80 -v GeoLite2-Country.mmdb:/db.mmdb geofilter -d ./db.mmdb -a US -t http://localhost:4000

 
    
