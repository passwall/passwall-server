# PassWall Server

**PassWall Server** is the core backend for open source password manager PassWall platform. Using this server, you can safely store your passwords and access them from anywhere. 

[![License](https://img.shields.io/github/license/pass-wall/passwall-server)](https://github.com/pass-wall/passwall-server/blob/master/LICENSE)
[![GitHub issues](https://img.shields.io/github/issues/pass-wall/passwall-server)](https://github.com/pass-wall/passwall-server/issues)
[![Build Status](https://travis-ci.org/pass-wall/passwall-server.svg?branch=master)](https://travis-ci.org/pass-wall/passwall-server) 
[![Coverage Status](https://coveralls.io/repos/github/pass-wall/passwall-server/badge.svg?branch=master)](https://coveralls.io/github/pass-wall/passwall-server?branch=master)
[![Docker Pull Status](https://img.shields.io/docker/pulls/passwall/passwall-server)](https://hub.docker.com/u/passwall/)  
[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy)

## Support
I promise all the coffee you have ordered will be spent on this project  
[![Become a Patron](https://www.yakuter.com/wp-content/yuklemeler/yakuter-patreon.png)](https://www.patreon.com/bePatron?u=33541638)

## Clients
PassWall can be used by these clients or you can write your own client by using [API Documentation](https://documenter.getpostman.com/view/3658426/SzYbyHXj)     
[PassWall Web](https://github.com/pass-wall/passwall-web)  
[PassWall Desktop](https://github.com/pass-wall/passwall-desktop)  
[PassWall Mobile](https://github.com/pass-wall/passwall-mobile)  

The screenshot of Passwall Desktop working with Passwall Server is as follows  
![PassWall Desktop Screenshot](https://www.yakuter.com/wp-content/yuklemeler/PassWall-Desktop-Screenshot.png "PassWall Desktop")

## API Documentation
API documentation available at:   
[Click to see at Public Postman](https://documenter.getpostman.com/view/3658426/SzYbyHXj)   

## DEMO
**Address:** https://passwall-server.herokuapp.com  
**Username:** passwall  
**Password:** password

## Database supoort
PassWall works with **PostgreSQL** databases. Settings required for connection to database are in **./store/config.yml**.

## What's possible with PassWall Server?
Currently, this project is focused on storing URL, username and password which is basically called **Login** at PassWall.

An admin can;  
- View and search logins
- Create login with automatically generated strong password
- Update login
- Delete login
- Import logins from other password managers
- Export logins as CSV format

## Authentication and Security
This server uses **JWT Token** to secure endpoints. So user must generate token with **/auth/signin** first. Then with generated token, all endpoints in API documentation can be reachable. 
  
User information for signin is in **config.yml** file.

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
go run ./cmd/passwall-server/main.go
```

The server uses config file end environment variables. If you want to set variables manually, just change **config-sample.yml** to **config.yml** in **store** folder.

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

## Import
There are different kinds of password managers. Almost all of them can export login information as CSV file. Here is an example CSV file (let's say example.csv).  
![example csv](https://www.yakuter.com/wp-content/yuklemeler/example-csv.png "Example CSV File")  
  
You need to fill the import form as below picture.  
![passwall-server import](https://www.yakuter.com/wp-content/yuklemeler/gpass-import-csv.png "Import Form and Request Example")

## Hello Contributors

1. Don't send too much commit at once. It will be easier for us to do a code review.

1. Be sure to take a look at the dev branch. The version I am working on is there.

1. First try to fix `// TODO:`s in the code.

1. Then you can contribute to the development by following the mile stones.

1. Don't mess with the user interface. The design guide has not been released yet.
