
message:
	protoc --go_out=. --go-grpc_out=. proto/*.proto

service:
	protoc --go_out=. --go-grpc_out=. proto/service.proto

clean:
	rm -rf pb/*.go

server:
	go run cmd/server/main.8080.go --port 8080

server2:
	go run cmd/server2/main.8081.go --port 8081

server3:
	go run cmd/server3/main.8082.go --port 8082

client:
	go run cmd/client/main.go -address 0.0.0.0:8080