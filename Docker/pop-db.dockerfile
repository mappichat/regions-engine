FROM golang:1.19

WORKDIR /usr/src/app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY ./src ./src
RUN go build -v -o /usr/local/bin/app ./src

ENV DB_STRING="host=postgres port=5432 user=postgres password=password dbname=postgres sslmode=disable"
ENV H3_TO_COUNTRIES=https://storage.googleapis.com/regions-data/test/h3ToCountry.json
ENV LEVEL_PATHS=https://storage.googleapis.com/regions-data/test/level0.json,https://storage.googleapis.com/regions-data/test/level1.json,https://storage.googleapis.com/regions-data/test/level2.json,https://storage.googleapis.com/regions-data/test/level3.json,https://storage.googleapis.com/regions-data/test/level4.json,https://storage.googleapis.com/regions-data/test/level5.json

CMD app dbwrite "${DB_STRING}" ${H3_TO_COUNTRIES} ${LEVEL_PATHS}
