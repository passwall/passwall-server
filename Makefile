build:
	docker-compose build
start:
	docker-compose start
stop:
	docker-compose down
run:
	docker-compose up -d
build-run:
	docker-compose up -d --build