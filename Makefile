.PHONY: lint vet test test-cover install-lint fmt help

all: lint vet test

help:
	@echo "Доступные команды сборки и автоматизации:"
	@echo "  make lint         - Запустить строгий статический анализ кодовой базы через golangci-lint"
	@echo "  make vet          - Проверить код стандартным официальным утилитарным анализатором go vet"
	@echo "  make test         - Прогнать все юнит-тесты корневого пакета библиотеки"
	@echo "  make test-cover   - Запустить тесты с расчетом покрытия и открыть интерактивный HTML-отчет"
	@echo "  make install-lint - Скомпилировать актуальную версию golangci-lint из исходников под текущий Go"
	@echo "  make fmt          - Отформатирование файлов кодовой базы по официальному стандарту Go"

fmt:
	@echo "==> Выравнивание синтаксиса и отступов через gofmt..."
	gofmt -w -s .

lint:
	@echo "==> Запуск комплексного анализа через golangci-lint..."
	golangci-lint run ./...

vet:
	@echo "==> Запуск официальной проверки go vet..."
	go vet ./...

test:
	@echo "==> Запуск изолированных unit-тестов..."
	go test -v -race .

test-cover:
	@echo "==> Сбор метрик атомарного покрытия кода..."
	go test -v -race -covermode=atomic -coverprofile=coverage.txt .

install-lint:
	@echo "==> Сборка golangci-lint v1.64.8 из исходников..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
