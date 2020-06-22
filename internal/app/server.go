package app

import (
	"encoding/base64"

	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// CreateServer creates a server and saves it to the store
func CreateServer(s storage.Store, dto *model.ServerDTO, schema string) (*model.Server, error) {
	createdServer, err := s.Servers().Save(model.ToServer(dto), schema)
	if err != nil {
		return nil, err
	}

	return createdServer, nil
}

// UpdateServer updates the server with the dto and applies the changes in the store
func UpdateServer(s storage.Store, server *model.Server, dto *model.ServerDTO, schema string) (*model.Server, error) {

	server.Title = dto.Title
	server.IP = dto.Title
	server.Username = dto.Username
	server.Password = dto.Password
	server.URL = dto.URL
	server.HostingUsername = dto.HostingUsername
	server.HostingPassword = dto.HostingPassword
	server.AdminUsername = dto.AdminUsername
	server.AdminPassword = dto.AdminPassword

	updatedServer, err := s.Servers().Save(server, schema)
	if err != nil {
		return nil, err
	}
	return updatedServer, nil
}

// DecryptServerPassword decrypts password
func DecryptServerPassword(s storage.Store, server *model.Server) (*model.Server, error) {
	passByte, _ := base64.StdEncoding.DecodeString(server.Password)
	server.Password = string(Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))

	return server, nil
}

// DecryptServerPasswords ...
// TODO: convert to pointers
func DecryptServerPasswords(serverList []model.Server) []model.Server {
	for i := range serverList {
		if serverList[i].Password == "" {
			continue
		}
		passByte, _ := base64.StdEncoding.DecodeString(serverList[i].Password)
		passB64 := string(Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))
		serverList[i].Password = passB64
	}
	return serverList
}
