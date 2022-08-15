protoc:
	protoc -I protos protos/usersinfo.proto --go_out=protos/. --go-grpc_out=protos/.
openssl-grpc:
	openssl req -x509 -newkey rsa:4096 -keyout certfiles/grpc/key.pem -out certfiles/grpc/cert.pem -sha256 -days 1280 -nodes -subj "/CN=localhost" -addext "subjectAltName = DNS:localhost"
openssl-general:
	openssl req -x509 -newkey rsa:4096 -keyout certfiles/general/key.pem -out certfiles/general/cert.pem -sha256 -days 1280 -nodes -subj "/CN=localhost" -addext "subjectAltName = DNS:localhost"
