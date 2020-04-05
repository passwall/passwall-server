# gpass

**gpass** is an open source password manager API written with Go. 

By using this API you can store your passwords whereever you want and manage easily event with just Postman etc.

## What's possible with gpass API?

Currently, gpass is focused on storing URL, username and password which is basicly called **Login** at gpass. 

An admin can;

- View all logins
- View a specific login
- Create a login with automatically generated strong password
- Update a login
- Delete login
    
API documentation available at: https://documenter.getpostman.com/view/3658426/SzYbyHXj

## Authentication

This API uses **Basic Auth** to secure endpoints. So do not forget to update **config.yml** for user information and add **Basic Auth authorization** to your requests from clients like **Postman**.

## Installation
Just change **config-sample.yml** to **config.yml** and update the content of this file for your usage. Then you can run API with standart command: 'go run main.go'