# /output should map to the volume you want to store the data (assuming it's on the same system as container)
# If where you're storing is a remote host, don't even bother setting a volume. The destination var
# should point to an scp destination.

FROM golang:1.19

WORKDIR /usr/src/app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY ./src ./src
RUN go build -v -o /usr/local/bin/app ./src

RUN mkdir /output

ENV RES=5
ENV COUNTRIES_GEOJSON_LOCATION=https://storage.googleapis.com/regions-data/countries.geojson
ENV POPMAP_LOCATION=https://storage.googleapis.com/regions-data/resolution5/popmap.json
ENV CONFIG_LOCATION=https://storage.googleapis.com/regions-data/resolution5/config.json

CMD app generate ${COUNTRIES_GEOJSON_LOCATION} -r ${RES} -o /output -p ${POPMAP_LOCATION} -c ${CONFIG_LOCATION}
