package main

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/passwall/passwall-server/internal/app"
	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	"github.com/passwall/passwall-server/pkg/constants"
)

func main() {
	cfg, err := config.SetupConfigDefaults(constants.ConfigPath, constants.ConfigName)
	if err != nil {
		log.Fatal(err)
	}

	db, err := storage.DBConn(&cfg.Database)
	if err != nil {
		log.Fatal(err)
	}

	s := storage.New(db)
	c := color.New(color.FgCyan)

	reader := bufio.NewReader(os.Stdin)
	c.Print("Enter Name: ")
	name, _ := reader.ReadString('\n')

	c.Print("Enter E-mail Address: ")
	email, _ := reader.ReadString('\n')

	c.Print("Enter Master Password: ")
	password, _ := reader.ReadString('\n')

	name = clearInput(name)
	email = clearInput(email)
	password = clearInput(password)

	if name == "" || email == "" || password == "" {
		log.Fatal("All fields are required.")
	}

	passwordHash := fmt.Sprintf("%x", newSHA256([]byte(password))[:])

	newUser := &model.UserDTO{
		Name:           name,
		Email:          email,
		MasterPassword: passwordHash,
	}

	createdUser, err := app.CreateUser(s, newUser)
	if err != nil {
		log.Fatal(err)
	}

	subscription := &model.Subscription{
		UserID: int(createdUser.ID),
		Email:  createdUser.Email,
		Status: "active",
		Type:   "pro",
	}

	_, err = s.Subscriptions().Save(subscription)
	if err != nil {
		log.Fatal(err)
	}

	color.Green("User created successfully.")
}

func clearInput(input string) string {
	return strings.TrimSpace(input)
}

func newSHA256(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}
