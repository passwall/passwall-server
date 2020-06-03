# PASSWALL CHANGELOG

## Version: [1.1.1] (2020-06---)
### Changed
- Now only supports PostgreSQL
### Add
- Secure notes feature
### Removed
- Removed SQLite and MySQL support

## Version: [1.1.0] (2020-05-03)
### Add
- Bank Account and Credit Card Categories
- net/http, mux router, negroni stack
- Security layer with middleware against XSS attacks
- public folder to serve static files on debian installation
- check password endpoint.
- Auto backup system with period config

### Security
- Access and Refresh Token usage implemented
- HS256 Signing method used on JWT

### Removed
- Gin framework

### Changed
- Move sqlite database name and path declaration to config file

## Version: [1.0.8] (2020-04-21)
### Add
- Search,Limit,Offset,Sort,Order query parameters to FindAll()
- Backup to ./store/passwall.bak file
- Restore from./store/passwall.bak file
  
### Changed
- Order BY to updated_at DESC
- Trim http://, https:// and www from URL's
- Create util and helper folder
- Refactor

## Version: [1.0.7] (2020-04-17)
### Added
- Refactored configuration. Now API accepts ENV variables.
- Generated passwall-api docker image and uploaded to Docker Hub

## Version: [1.0.6] (2020-04-15)
### Changed
- Docker file for store folder
- Return URL's host info only


## Version: [1.0.5] (2020-04-12)
### Added
- JWT token for authentication
- signin, refresh and check endpoints under auth group
- secret key in config.yml to use in JWT token generation
- timeout key in config.yml to define duration of JWT token

## Version: [1.0.4] (2020-04-11)
### Added
- Export logins feature
- Get Method test for API GET endpoints
- Checking for Limit (5) and Offset (0)
- Checking at if record exist on import
- Case insensitive search posgres
- POST generate-password endpoint
- Frontend written with React Native and Nextjs
- Travis CI
- Badges to README file including code coverage
### Fixed
- Excluded soft deleted items from total and filtered count number

## Version: [1.0.3] (2020-04-07)
### Added
- Import logins feature
- docker-compose.yml
### Fixed
- login.Password database recording bug for postgresql and mysql
- Upload imported file bug

## Version: [1.0.2] (2020-04-07)
### Changed
- Folder structure of controller


## Version: [1.0.1] (2020-04-06)
### Added
- Two way strong encryption to stored passwords
- Passphrase key to config file
- Docker file

## Version: [1.0.0] (2020-04-05)
- Initial commit
  
<!-- ### Added
### Changed
### Removed
### Fixed
### Deprecated
### Security -->