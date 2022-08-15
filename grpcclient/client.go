package grpcclient

import (
	"context"
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
func (u *UsersInfo) Get(key string) (bool, Keys) {
	creds, err := credentials.NewClientTLSFromFile("certfiles/cert.pem", "")
	if err != nil {
		u.log.Println(err.Error())
	}
	dial := fmt.Sprintf("%s:%d", settings.GRPCUsersInfoSettings.Items.Host, settings.GRPCUsersInfoSettings.Items.Port)
	con, err := grpc.Dial(dial, grpc.WithTransportCredentials(creds))
	if err != nil {
		u.log.Println(err.Error())
	}
	defer con.Close()

	// UsersInfo Client
	info := protos.NewUsersInfoClient(con)

	request := &protos.UsersInfoRequest{
		Key: key,
	}
	response, err := info.Get(context.Background(), request)
	if err != nil {
		u.log.Println(err.Error())
	}

	keys := Keys{}

	auth := response.GetAuth()
	keys.Twitch = response.GetTwitchKey()
	keys.Youtube = response.GetYoutubeKey()
	keys.Aparat = response.GetAparatKey()

	return auth, keys
}
