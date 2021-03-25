# go-whosonfirst-spatial-pip-sqlite

## Important

This is work in progress. Documentation to follow.

## Tools

```
$> make cli
go build -mod vendor -o bin/query cmd/query/main.go
```

### query

```
$> ./bin/query -h
  -alternate-geometry value
    	One or more alternate geometry labels (wof:alt_label) values to filter results by.
  -cessation-date string
    	A valid EDTF date string.
  -custom-placetypes string
    	A JSON-encoded string containing custom placetypes defined using the syntax described in the whosonfirst/go-whosonfirst-placetypes repository.
  -enable-custom-placetypes
    	Enable wof:placetype values that are not explicitly defined in the whosonfirst/go-whosonfirst-placetypes repository.
  -enable-geojson
    	...
  -geometries string
    	Valid options are: all, alt, default. (default "all")
  -inception-date string
    	A valid EDTF date string.
  -is-ceased value
    	One or more existential flags (-1, 0, 1) to filter results by.
  -is-current value
    	One or more existential flags (-1, 0, 1) to filter results by.
  -is-deprecated value
    	One or more existential flags (-1, 0, 1) to filter results by.
  -is-superseded value
    	One or more existential flags (-1, 0, 1) to filter results by.
  -is-superseding value
    	One or more existential flags (-1, 0, 1) to filter results by.
  -is-wof
    	Input data is WOF-flavoured GeoJSON. (Pass a value of '0' or 'false' if you need to index non-WOF documents. (default true)
  -iterator-uri string
    	A valid whosonfirst/go-whosonfirst-iterate/emitter URI. Supported schemes are: directory://, featurecollection://, file://, filelist://, geojsonl://, repo://. (default "repo://")
  -latitude float
    	A valid latitude.
  -longitude float
    	A valid longitude.
  -mode string
    	... (default "cli")
  -placetype value
    	One or more place types to filter results by.
  -properties-reader-uri string
    	A valid whosonfirst/go-reader.Reader URI. Available options are: [file:// fs:// null://]
  -property value
    	One or more Who's On First properties to append to each result.
  -server-uri string
    	... (default "http://localhost:8080")
  -spatial-database-uri string
    	A valid whosonfirst/go-whosonfirst-spatial/data.SpatialDatabase URI. options are: [sqlite://]
  -verbose
    	Be chatty.
```

#### Command line

```
$> ./bin/query \
	-spatial-database-uri 'sqlite://?dsn=/usr/local/data/arch.db' \
	-latitude 37.616951 \
	-longitude -122.383747 \
	-is-current 1

| jq '.["places"][]["wof:id"]'

"1729792685"
"1729792433"
```

#### Server

```
$> ./bin/query -mode server -spatial-database-uri 'sqlite://?dsn=/usr/local/data/arch.db'
```

And in another terminal:

```
$> curl -s -XPOST \
	http://localhost:8080/ \
	-d '{"latitude":37.616951,"longitude":-122.383747,"is_current":[1]}' \

| jq '.["places"][]["wof:id"]'

"1729792685"
"1729792433"
```

When the query tool is run in `server` mode you are expected to post a valid [api.PointInPolygonRequest](https://github.com/whosonfirst/go-whosonfirst-spatial/blob/main/api/pointinpolygon.go#L11) data structure as the HTTP `POST` body.

#### Lambda (using container images)

The easiest way to get started is to run the `docker` Makefile target passing in a `DATABASE` parameter pointing to the SQLite database you want to bundle with this container image.

As of this writing the database itself is stored in the container as `/usr/local/data/query.db`. Eventually it will be stored with its original filename.

```
$> make docker-query DATABASE=/usr/local/data/arch.db
docker build --build-arg DATABASE=query.db -f Dockerfile.query -t point-in-polygon .

...Docker stuff

=> [ 5/10] RUN mkdir /usr/local/data
=> [ 6/10] COPY query.db /usr/local/data/query.db

...More Docker stuff

=> naming to docker.io/library/point-in-polygon   
```

##### Running locally

```
$> docker run -e PIP_MODE=lambda -e PIP_SPATIAL_DATABASE_URI=sqlite://?dsn=/usr/local/data/query.db -p 9000:8080 point-in-polygon:latest /main
time="2021-03-11T01:19:37.994" level=info msg="exec '/main' (cwd=/go, handler=)"
```

And then in another terminal:

```
$> curl -s -XPOST \
	"http://localhost:9000/2015-03-31/functions/function/invocations" \
	-d '{"latitude":37.616951,"longitude":-122.383747,"is_current":[1]}' \

| jq '.["places"][]["wof:id"]'

"1729792685"
"1729792433"
```

When the query tool is run in `lambda` mode you are expected to post a valid [api.PointInPolygonRequest](https://github.com/whosonfirst/go-whosonfirst-spatial/blob/main/api/pointinpolygon.go#L11) data structure as the HTTP `POST` body.

##### Running in AWS

Update your container image to a your AWS ECS repository. Create a new AWS Lambda function and configure it to use your container.

Ensure the following image configuration variables are assigned:

| Name | Value |
| --- | --- |
| CMD override | /main |

Ensure the following environment variables are assigned:

| Name | Value |
| --- | --- |
| PIP_MODE | lambda |
| PIP_SPATIAL_URI | sqlite://?dsn=/usr/local/data/query.db |

Create a test like this and invoke it:

```
{
  "latitude": 37.616951,
  "longitude": -122.383747,
  "is_current": [
    1
  ]
}
```

When the query tool is run in `lambda` mode you are expected to post a valid [api.PointInPolygonRequest](https://github.com/whosonfirst/go-whosonfirst-spatial/blob/main/api/pointinpolygon.go#L11) data structure as the HTTP `POST` body.

##### Running in AWS with API Gateway

Ensure the following environment variables are assigned:

| Name | Value |
| --- | --- |
| PIP_MODE | server |
| PIP_SERVER_URI | lambda:// |
| PIP_SPATIAL_URI | sqlite://?dsn=/usr/local/data/query.db |

_Complete API Gateway documentation to be written_

```
$> curl -s -XPOST \
	https://{PREFIX}.execute-api.us-west-2.amazonaws.com/pip \
	-d '{"latitude":37.616951,"longitude":-122.383747,"is_current":[1]}' \

| jq '.["places"][]["wof:id"]'

"1729792433"
"1729792685"
```

When the query tool is run in `server` mode (which is what you're doing when using an Lambda + API Gateway setup) you are expected to post a valid [api.PointInPolygonRequest](https://github.com/whosonfirst/go-whosonfirst-spatial/blob/main/api/pointinpolygon.go#L11) data structure as the HTTP `POST` body.

## See also

* https://github.com/whosonfirst/go-whosonfirst-spatial-sqlite
* https://github.com/whosonfirst/go-whosonfirst-spatial-pip
* https://github.com/whosonfirst/go-whosonfirst-spatial