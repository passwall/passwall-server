package api

/*
type TestSuiteEnv struct {
	suite.Suite
	db   *storage.Database
	gorm *gorm.DB
	// conf *config.Configuration
}

// Testler başlamadan önce çalıştırılıyor
func (suite *TestSuiteEnv) SetupSuite() {

	// 1. Set config env variable to "dev"
	viper.Set("server.env", "dev")

	// 2. Set mock db credentials
	mockDBConfig := &config.DatabaseConfiguration{
		Name:     "passwall",
		Username: "postgres",
		Password: "postgres",
		Host:     "localhost",
		Port:     "5432",
		LogMode:  true,
	}

	// 3. Create db connection
	mockDB, err := storage.DBConn(mockDBConfig)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}

	// 4. Create new storage
	s := storage.New(mockDB)
	suite.db = s
	suite.gorm = mockDB

	// 5. Migrate system tables: subscriptions, tokens, users
	app.MigrateSystemTables(s)

	// 6. Set dummy user information
	userDTO := &model.UserDTO{
		Name:           "Test User",
		Email:          "test@passwall.io",
		MasterPassword: "123456",
	}

	// 7. Create dummy user
	createdUser, err := app.CreateUser(s, userDTO)
	if err != nil {
		suite.T().Error(err)
	}

	// 8. Update user once to generate schema
	updatedUser, err := app.GenerateSchema(s, createdUser)
	if err != nil {
		suite.T().Error(err)
	}

	// 9. Create user schema and tables
	err = s.Users().CreateSchema(updatedUser.Schema)
	if err != nil {
		suite.T().Error(err)
	}

	// 10. Create user tables in user schema
	app.MigrateUserTables(s, updatedUser.Schema)

}

// Her bir testten sonra çalıştırılıyor
func (suite *TestSuiteEnv) TearDownTest() {
	// database.ClearTables()
}

// Bütün testler bittikten sonra çalıştırılıyor
func (suite *TestSuiteEnv) TearDownSuite() {
	suite.gorm.Close()
}

// This gets run automatically by `go test` so we call `suite.Run` inside it
func TestSuite(t *testing.T) {
	// This is what actually runs our suite
	suite.Run(t, new(TestSuiteEnv))
}

*/
