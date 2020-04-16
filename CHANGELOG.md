# PASSWALL CHANGELOG

## Version: [-.-.-] (2020-04---)
### Added
- If there is no config.yml, reads from default ENV variables defined in /pkg/config/configuration.go

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