# Инструкция по развертыванию SandTracker на Docker

## Структура проекта

```
/workspace/
├── backend/           # Go бэкенд приложение
│   ├── main.go
│   ├── go.mod
│   └── go.sum
├── frontend/          # Статические файлы фронтенда
│   ├── index.html
│   ├── login.html
│   └── favicon.png
├── nginx/             # Конфигурация Nginx
│   └── default.conf
├── scripts/           # SQL скрипты для инициализации БД
│   └── init.sql
├── Dockerfile         # Dockerfile для бэкенда
├── Dockerfile.db      # Dockerfile для PostgreSQL
├── Dockerfile.nginx   # Dockerfile для Nginx
├── docker-compose.yml # Оркестрация всех сервисов
└── .env               # Переменные окружения
```

## Быстрый старт

### 1. Запуск всех сервисов

```bash
docker compose up -d --build
```

### 2. Проверка статуса

```bash
docker compose ps
```

### 3. Просмотр логов

```bash
docker compose logs -f
```

### 4. Остановка сервисов

```bash
docker compose down
```

### 5. Остановка с удалением томов (данные БД будут удалены!)

```bash
docker compose down -v
```

## Доступ к сервисам

- **Веб-интерфейс**: http://localhost:80
- **PostgreSQL**: localhost:5432
- **Backend API**: http://localhost:8080

## Пользователи по умолчанию

| Username | Password   |
|----------|------------|
| Stalker  | 16084636   |
| Bob      | z53Z2OsJ1  |
| Apple    | z53Z2OsJ2  |
| Admin    | z53Z2OsJ67 |

## Настройка переменных окружения

Отредактируйте файл `.env` для изменения конфигурации:

```env
# Database configuration
POSTGRES_DB=sandtracker
POSTGRES_USER=sanduser
POSTGRES_PASSWORD=sandpass123

# Backend configuration
DB_HOST=postgres
DB_PORT=5432
DB_USER=sanduser
DB_PASSWORD=sandpass123
DB_NAME=sandtracker
BACKEND_PORT=8080

# Nginx configuration
NGINX_PORT=80
```

## Архитектура

1. **PostgreSQL** - база данных для хранения пользователей, списков и сессий
2. **Backend (Go)** - REST API сервер
3. **Nginx** - веб-сервер для раздачи статики и проксирования запросов к API

Все сервисы работают в изолированной Docker сети `sand_network`.

## Решение проблем

### Бэкенд не подключается к базе данных

Проверьте логи бэкенда:
```bash
docker compose logs backend
```

Убедитесь, что PostgreSQL запустился:
```bash
docker compose logs postgres
```

### Неправильно работает healthcheck

Перезапустите сервис PostgreSQL:
```bash
docker compose restart postgres
```

### Сброс базы данных

```bash
docker compose down -v
docker compose up -d --build
```

## Развертывание на Ubuntu Server

1. Установите Docker и Docker Compose:
```bash
sudo apt update
sudo apt install docker.io docker-compose-plugin -y
```

2. Скопируйте файлы проекта на сервер

3. Запустите сервисы:
```bash
cd /path/to/project
docker compose up -d --build
```

4. Откройте порт 80 в фаерволе:
```bash
sudo ufw allow 80/tcp
```
