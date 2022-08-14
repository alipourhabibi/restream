protoc:
	protoc -I protos protos/usersinfo.proto --go_out=protos/. --go-grpc_out=protos/.
