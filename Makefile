# Go ahead and try using a .env file :)
include .env
export

# Environment variables. These can optionally be set in your environment.
# If not set, they will default to the values seen here.
RES := $(or $(RES),5)
DIR := $(or $(DIR),./data/resolution${RES})
DB_STRING := $(or $(DB_STRING),"host=localhost port=5432 user=postgres password=password dbname=postgres sslmode=disable")
PORT := $(or $(PORT),8080)
COUNTRIES_GEOJSON_LOCATION := $(or $(COUNTRIES_GEOJSON_LOCATION),https://storage.googleapis.com/regions-data/countries.geojson)
POPMAP_LOCATION := $(or $(POPMAP_LOCATION),https://storage.googleapis.com/regions-data/resolution5/popmap.json)
CONFIG_LOCATION := $(or $(CONFIG_LOCATION),https://storage.googleapis.com/regions-data/resolution5/config.json)
DATA_DESTINATION := $(or $(DATA_DESTINATION),./output)

generate:
	go run ./src/main.go generate ${COUNTRIES_GEOJSON_LOCATION} \
	-r ${RES} \
	-o ${DATA_DESTINATION} \
	-p ${POPMAP_LOCATION} \
	-c ${CONFIG_LOCATION}

docker-generate:
	export RES=${RES}; \
	export COUNTRIES_GEOJSON_LOCATION=${COUNTRIES_GEOJSON_LOCATION}; \
	export POPMAP_LOCATION=${POPMAP_LOCATION}; \
	export CONFIG_LOCATION=${CONFIG_LOCATION}; \
	export DATA_DESTINATION=${DATA_DESTINATION}; \
	docker-compose up generate --build

serve:
	go run ./src/main.go serve ${DIR} \
	-p ${PORT}

build:
	echo ${RES}
	go build -o ./bin/region-engine.bin ./src/main.go

build-generate:
	./bin/region-engine.bin generate ${COUNTRIES_GEOJSON_LOCATION} \
	-r ${RES} \
	-o ${DATA_DESTINATION} \
	-p ${POPMAP_LOCATION} \
	-c ${CONFIG_LOCATION}

build-serve:
	./bin/region-engine.bin serve ${DIR} \
	-p ${PORT}

populate-db:
	./bin/region-engine.bin dbwrite ${DIR} ${DB_STRING}

db:
	docker-compose up adminer postgres
