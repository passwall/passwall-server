# Passwall Server

**Passwall Server**'a hoşgeldiniz. Bu doküman Passwall Server'ın development ortamını nasıl kuracağınızı anlatmaktadır. 

## Gereksinimler

1. Git
2. Docker
3. Docker-compose v2

## Clone Github Repo

Öncelikle Passwall Server'ın github reposunu lokalimize clone'luyoruz ve indirdiğimiz klasörün içine giriyoruz.
```bash
 git clone https://github.com/passwall/passwall-server.git
 cd passwall-server
 ```

## Run Postgresql

Passwall Server, veritabanı olarak **Postgresql** kullanmaktadır. Postgresql'i hazır docker compose dosyasını kullanarak aşağıdaki komut ile ayağa kaldırıyoruz. Postgresql veritabanı bilgilerini (kullanıcı adı, parola vb.) bu dosyayı bir metin editörüyle açarak görebilir ve herhangi bir Postgresql aracıyla bu bilgileri kullanarak bağlanabilirsiniz.
```bash
docker compose -f docker-compose-postgres.yml up -d --remove-orphans
```

## Run Passwall

Passwall Server'ı yine aynı reponun içindeyken aşağıdaki komut ile docker üzerinde ayağa kaldırabilirsiniz.

```bash
docker compose -f docker-compose-passwall.yml up -d --remove-orphans
```

## Create User

Terminal'den Passwall Server'ın CLI uygulamasını kullanarak aşağıdaki komut ile kullanıcı oluşturabilirsiniz. Postman dokümanları ile uyumlu olması açısından test kullanıcısını aşağıdaki bilgilerle oluşturunuz.

```bash
docker exec -it passwall-server /app/passwall-cli
# Name Surname: Test Passwall
# E-mail Address: test@test.com
# Master Password: 123456
```