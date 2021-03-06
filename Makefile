cli:
	go build -mod vendor -o bin/query cmd/query/main.go

docker:
	cp $(DATABASE) query.db
	docker build --build-arg DATABASE=query.db -f Dockerfile.query -t point-in-polygon .
	rm query.db

# test with:
# curl -XPOST "http://localhost:9000/2015-03-31/functions/function/invocations" -d '{"latitude":37.616951,"longitude":-122.383747}'
# curl -XPOST "http://localhost:9000/2015-03-31/functions/function/invocations" -d '{"latitude":37.616951,"longitude":-122.383747,"is_current":[1]}'

lambda:
	docker run -e PIP_MODE=lambda -e PIP_SPATIAL_DATABASE_URI=sqlite://?dsn=/usr/local/data/query.db -p 9000:8080 point-in-polygon:latest /main
