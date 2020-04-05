package controller

/* func Upload(c *gin.Context) {
	db = database.GetDB()

	var login model.Login

	c.JSON(200, login)
}

func Read() {
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
			URL:      dizi[4],
			Username: dizi[5],
			Password: dizi[1],
		}

		db.Create(&login)
		// fmt.Println(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
} */
