protoc:
	protoc -I protos protos/usersinfo.proto --go_out=protos/. --go-grpc_out=protos/.
openssl:
	openssl req -x509 -newkey rsa:4096 -keyout certfiles/key.pem -out certfiles/cert.pem -sha256 -days 1280 -nodes -subj "/CN=localhost" -addext "subjectAltName = DNS:localhost"
