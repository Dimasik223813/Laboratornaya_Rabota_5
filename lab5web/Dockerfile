# Используем официальный образ Golang
FROM golang:1.25.0-alpine

WORKDIR /app

# Копируем файлы модулей и устанавливаем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходники
COPY . .

# Сборка бинарника
RUN go build -o ginapp .

# Итоговый образ (самый простой вариант)
CMD ["/app/ginapp"]
