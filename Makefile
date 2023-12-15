
message:
	protoc --go_out=. --go-grpc_out=. proto/*.proto

service:
	protoc --go_out=. --go-grpc_out=. proto/service.proto

clean:
	rm -rf pb/*.go

server:
	redis-cli flushall
	go run cmd/server/main.8080.go --prt 8081

server2:
	redis-cli flushall
	go run cmd/server/main.8080.go --prt 8082

client:
	go run cmd/client/main.go