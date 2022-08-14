package settings

import (
	gcfg "gopkg.in/gcfg.v1"
)

type server struct {
	Items serverItems `gcfg:"server"`
}

type serverItems struct {
	RunMode string `gcfg:"RunMode"`
	Port    int    `gcfg:"Port"`
}

type gRPCUsersInfo struct {
	Items gRPCUsersInfoItems `gcfg:"grpcusersinfo"`
}

type gRPCUsersInfoItems struct {
	Host string `gcfg:"Host"`
	Port int    `gcfg:"Port"`
}

// ServerSettings Holds datas for settings in conf/conf.ini in server section
var ServerSettings server

// GRPCUsersInfoSettings Holds datas for settings in conf/conf.ini in grpc_usersinfo section
var GRPCUsersInfoSettings gRPCUsersInfo

// SetUp imports settings data from configure file to corresponding global variables
// that are defined in this package
func SetUp() {
	gcfg.ReadFileInto(&ServerSettings, "./conf/conf.ini")
	gcfg.ReadFileInto(&GRPCUsersInfoSettings, "./conf/conf.ini")
}
