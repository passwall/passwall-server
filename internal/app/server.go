package app

import (
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
)

// CreateServer creates a server and saves it to the store
func CreateServer(s storage.Store, dto *model.ServerDTO, schema string) (*model.Server, error) {
	rawModel := model.ToServer(dto)
	encModel := EncryptModel(rawModel)

	createdServer, err := s.Servers().Save(encModel.(*model.Server), schema)
	if err != nil {
		return nil, err
	}

	return createdServer, nil
}

// UpdateServer updates the server with the dto and applies the changes in the store
func UpdateServer(s storage.Store, server *model.Server, dto *model.ServerDTO, schema string) (*model.Server, error) {
	rawModel := model.ToServer(dto)
	encModel := EncryptModel(rawModel).(*model.Server)

	server.Title = encModel.Title
	server.IP = encModel.IP
	server.Username = encModel.Username
	server.Password = encModel.Password
	server.URL = encModel.URL
	server.HostingUsername = encModel.HostingUsername
	server.HostingPassword = encModel.HostingPassword
	server.AdminUsername = encModel.AdminUsername
	server.AdminPassword = encModel.AdminPassword

	updatedServer, err := s.Servers().Save(server, schema)
	if err != nil {
		return nil, err
	}
	return updatedServer, nil
}
