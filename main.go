package main

import (
	"bufio"
	"gpass/model"
	"gpass/pkg/config"
	"gpass/pkg/database"
	"gpass/pkg/router"
	"log"
	"os"
	"strings"
)

func init() {
	config.Setup()
	database.Setup()
}

func main() {
	config := config.GetConfig()

	r := router.Setup()
	r.Run("127.0.0.1:" + config.Server.Port)
}

func read() {
	db := database.GetDB()
	file, err := os.Open("./yedek.csv")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		dizi := strings.Split(scanner.Text(), ",")
		login := model.Login{
			URL:      dizi[4], // 1password URL field
			Username: dizi[5], // 1password username field
			Password: dizi[1], // 1password password field
		}

		db.Create(&login)
		// fmt.Println(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}
