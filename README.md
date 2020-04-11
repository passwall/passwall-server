# gpass

![GitHub](https://img.shields.io/github/license/yakuter/gpass)
![GitHub issues](https://img.shields.io/github/issues/yakuter/gpass)
[![Build Status](https://travis-ci.org/yakuter/gpass.svg?branch=master)](https://travis-ci.org/yakuter/gpass) 
[![Coverage Status](https://coveralls.io/repos/github/yakuter/gpass/badge.svg?branch=master)](https://coveralls.io/github/yakuter/gpass?branch=master)

**gpass** is an open source password manager API written with Go.

By using this API you can store your passwords wherever you want and manage easily event with just Postman etc.

## What's possible with gpass API?

Currently, gpass is focused on storing URL, username and password which is basically called **Login** at gpass.

An admin can;

- Sign in and Refresh token
- View all logins
- View a specific login
- Create login with automatically generated strong password
- Update login
- Delete login


## API Documentation
API documentation available at:   
[Click to see at Public Postman Templates](https://documenter.getpostman.com/view/3658426/SzYbyHXj)  
[Clidk to download Postman JSON file](https://www.yakuter.com/wp-content/yuklemeler/gpass_postman_collection.json_.zip)

## Authentication

This API uses **JWT Token** to secure endpoints. So user must generate token with /auth/signin first. Then with generated token, all endpoints in API documentation can be reachable.  
  
User information for signin is in **config.yml** file.

## Development usage
Just change **config-sample.yml** to **config.yml** in **store** folder and update the content of this file for your usage. Then you can run API with standard command:

```
go run main.go
```

## docker-compose

You can start gpass with a database by one line command:

**P.S: You should uncomment database service sections**

```
docker-compose up --build
```

## Docker usage
First get into you project folder. Then:

To build
```
docker build -t gpass .
```

To run
```
cp ./store/config-sample.yml ./store/config.yml
docker run --name gpass --rm -v $(pwd)/store:/app/store -p 3625:3625 gpass
```

To store persistent data (config.yml and gpass.db)
```
mkdir $HOME/docker/volumes/gpass
cp ./store/config-sample.yml $HOME/docker/volumes/gpass/config.yml
docker run --name gpass -d --restart=always -v $HOME/docker/volumes/gpass:/app/store -p 3625:3625 gpass
```

## Import
There are different kinds of password managers. Almost all of them can export login information as CSV file. Here is an example CSV file (let's say example.csv).  
![example csv](https://www.yakuter.com/wp-content/yuklemeler/example-csv.png "Example CSV File")  
  
You need to fill the import form as below picture.  
![gpass import](https://www.yakuter.com/wp-content/yuklemeler/gpass-import-csv.png "Import Form and Request Example")
