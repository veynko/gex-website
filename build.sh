#!/bin/bash

# Скрипт сборки для различных архитектур
# Поддерживает Orange Pi (ARM64) и другие платформы

set -e

# Определяем целевую архитектуру
TARGET_OS=${1:-linux}
TARGET_ARCH=${2:-arm64}

echo "Сборка GEX Dashboard для $TARGET_OS/$TARGET_ARCH..."

# Устанавливаем переменные окружения для кросс-компиляции
export GOOS=$TARGET_OS
export GOARCH=$TARGET_ARCH
export CGO_ENABLED=0

# Создаём директорию для сборки
BUILD_DIR="build/${TARGET_OS}_${TARGET_ARCH}"
mkdir -p $BUILD_DIR

# Сборка
go build -ldflags="-w -s" -o $BUILD_DIR/gex-dashboard .

# Копируем дополнительные файлы
cp install.sh $BUILD_DIR/
cp -r static $BUILD_DIR/static
cp README.md $BUILD_DIR/ 2>/dev/null || true
chmod +x $BUILD_DIR/install.sh

# Создаём архив
cd build
tar -czf gex-dashboard-${TARGET_OS}_${TARGET_ARCH}.tar.gz ${TARGET_OS}_${TARGET_ARCH}/
cd ..

echo "Сборка завершена: build/gex-dashboard-${TARGET_OS}_${TARGET_ARCH}.tar.gz"

# Информация о сборке
ls -lh $BUILD_DIR/gex-dashboard
file $BUILD_DIR/gex-dashboard
