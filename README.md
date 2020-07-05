# PassWall Server

**PassWall Server** is the core backend for open source password manager PassWall platform. Using this server, you can safely store your passwords and access them from anywhere. 

[![License](https://img.shields.io/github/license/passwall/passwall-server)](https://github.com/passwall/passwall-server/blob/master/LICENSE)
[![GitHub issues](https://img.shields.io/github/issues/passwall/passwall-server)](https://github.com/passwall/passwall-server/issues)
[![Build Status](https://travis-ci.org/passwall/passwall-server.svg?branch=master)](https://travis-ci.org/passwall/passwall-server) 
[![Coverage Status](https://coveralls.io/repos/github/passwall/passwall-server/badge.svg?branch=master)](https://coveralls.io/github/passwall/passwall-server?branch=master)
[![Docker Pull Status](https://img.shields.io/docker/pulls/passwall/passwall-server)](https://hub.docker.com/u/passwall/)  
[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy)

## Support
I promise all the coffee you have ordered will be spent on this project  
[![Become a Patron](https://www.yakuter.com/wp-content/yuklemeler/yakuter-patreon.png)](https://www.patreon.com/bePatron?u=33541638)

## Clients
**PassWall Server** can be used with [**PassWall Desktop**](https://github.com/passwall/passwall-desktop)

<p align="center">
    <img src="https://www.yakuter.com/wp-content/yuklemeler/passwall-screenshot.png" alt="" width="600" height="425" />
</p>

## API Documentation
API documentation available at [Postman Public Directory](https://documenter.getpostman.com/view/3658426/SzYbyHXj)   

## Database support
PassWall works with **PostgreSQL** databases. 

## Configuration
When PassWall Server starts, it automatically generates **config.yml** in the folders below:  
**MacOS:** $HOME/Library/Application Support/passwall-server  
**Windows:** $APPDATA/passwall-server  
**Linux:** $HOME/.config/passwall-server  


## Security
1. PassWall uses The Advanced Encryption Standard (AES) encryption algorithm with Galois/Counter Mode (GCM) symmetric-key cryptographic mode. Passwords encrypted with AES can only be decrypted with the passphrase defined in the **config.yml** file.

2. Endpoints are protected with security middlewares against attacks like XSS.

3. Against SQL injection, PassWall uses Gorm package to handle database queries which clears all queries.

4. There is rate limiter for signin attempts against brute force attacks.

## Environment Variables
These environment variables are accepted:

**Server Variables:**
- PORT
- PW_SERVER_USERNAME
- PW_SERVER_PASSWORD
- PW_SERVER_PASSPHRASE
- PW_SERVER_SECRET
- PW_SERVER_TIMEOUT  
- PW_SERVER_GENERATED_PASSWORD_LENGTH 
- PW_SERVER_ACCESS_TOKEN_EXPIRE_DURATION
- PW_SERVER_REFRESH_TOKEN_EXPIRE_DURATION 
  
**Database Variables**
- PW_DB_NAME
- PW_DB_USERNAME
- PW_DB_PASSWORD
- PW_DB_HOST
- PW_DB_PORT
- PW_DB_LOG_MODE

**Backup Variables**
- PW_BACKUP_FOLDER
- PW_BACKUP_ROTATION
- PW_BACKUP_PERIOD

## Development usage
Install Go to your computer. Pull the server repo. Execute the command in server folder.

```
go run ./cmd/passwall-server
```

## Docker

```
docker-compose up --build
```
or in project folder
```
docker pull passwall/passwall-server
cp ./store/config-sample.yml ./store/config.yml
docker run --name passwall-server --rm -v $(pwd)/store:/app/store -p 3625:3625 passwall/passwall-server
```

## Hello Contributors

1. Don't send too much commit at once. It will be easier for us to do a code review.

1. Be sure to take a look at the dev branch. The version I am working on is there.

1. First try to fix `// TODO:`s in the code.

1. Then you can contribute to the development by following the mile stones.

1. Don't mess with the user interface. The design guide has not been released yet.
