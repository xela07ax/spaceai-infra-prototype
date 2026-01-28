# Переменные проекта
MODULE_NAME := github.com/xela07ax/spaceai-infra-prototype
PROTO_SRC := api/connector/v1
PROTO_OUT := pkg/api/connector/v1

.PHONY: all gen-proto test build clean help db-shell

help: ## Показать справку
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

gen-proto: ## Сгенерировать Go-код из .proto файлов
	@echo "Generating Protobuf files..."
	@mkdir -p $(PROTO_OUT)
	protoc --proto_path=$(PROTO_SRC) \
		--go_out=$(PROTO_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_OUT) --go-grpc_opt=paths=source_relative \
		$(PROTO_SRC)/*.proto
	@echo "Done!"

test: ## Запустить unit-тесты
	go test -v -race ./internal/...

build: gen-proto ## Собрать бинарник UAG
	go build -o bin/uag cmd/uag/main.go

deps: ## Установить необходимые инструменты генерации
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

clean: ## Очистить сгенерированные файлы и бинарники
	rm -rf bin/
	find pkg/api -name "*.pb.go" -delete

db-shell: ## Вход в БД через сервис, даже если container_name изменится
	docker-compose exec postgres psql -U devit_user -d devit_db