package grpcclient

import (
	"log"

	protos "github.com/alipourhabibi/restream/protos/usersinfo"
	"github.com/alipourhabibi/restream/settings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Client is a gobal variable we use to access rpc methods
var Client protos.UsersInfoClient

// A UsersInfo struct Consists of a logger field and a SetUp method
type UsersInfo struct {
	log *log.Logger
}

// NewUsersInfo returns a new instance of &UsersInfo with given logger
func NewUsersInfo(log *log.Logger) *UsersInfo {
	return &UsersInfo{log: log}
}

// SetUp method sets up NewUsersInfoClient interface as global variable client to
// be used later
func (u *UsersInfo) SetUp() {
	creds, err := credentials.NewClientTLSFromFile("certfiles/cert.pem", "")
	if err != nil {
		u.log.Println(err.Error())
	}
	con, err := grpc.Dial(settings.GRPCUsersInfoSettings.Items.Host, grpc.WithTransportCredentials(creds))
	if err != nil {
		u.log.Println(err.Error())
	}
	defer con.Close()

	// UsersInfo Client
	Client = protos.NewUsersInfoClient(con)
}
