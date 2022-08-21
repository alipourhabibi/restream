package grpcclient

import (
	"fmt"
	"log"

	protos "github.com/alipourhabibi/restream/protos/usersinfo"
	"github.com/alipourhabibi/restream/settings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Keys struct {
	Twitch  string
	Youtube string
	Aparat  string
}

// A UsersInfo struct Consists of a logger field and a SetUp method
type UsersInfo struct {
	log *log.Logger
}

// NewUsersInfo returns a new instance of &UsersInfo with given logger
func NewUsersInfo(log *log.Logger) *UsersInfo {
	return &UsersInfo{log: log}
}

// Get method is used to get user's data including keys from grpc server
func (u *UsersInfo) GetClient() protos.UsersInfoClient {
	creds, err := credentials.NewClientTLSFromFile("certfiles/grpc/cert.pem", "")
	if err != nil {
		u.log.Println(err.Error())
	}
	dial := fmt.Sprintf("%s:%d", settings.GRPCUsersInfoSettings.Items.Host, settings.GRPCUsersInfoSettings.Items.Port)
	con, err := grpc.Dial(dial, grpc.WithTransportCredentials(creds))
	if err != nil {
		u.log.Println(err.Error())
	}

	// UsersInfo Client
	client := protos.NewUsersInfoClient(con)

	return client
}
