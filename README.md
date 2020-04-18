docker build -f Dockerfile -t geofilter .


docker run -ti --rm -p 8080:80 -v C:/Dev/Bhoomi/assets/GeoLite2-Country.mmdb:/app/db.mmdb geofilter -d ./db.mmdb -a RU -t http://localhost:4000

 
    